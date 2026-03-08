package tee

import (
	"vectorspace/enclave"
	"vectorspace/platform"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// TEEProxy manages periodic sync to the real enclave and caches attestation.
type TEEProxy struct {
	client   *VsockClient
	registry *platform.PositionRegistry
	budgets  *platform.BudgetTracker
	stopCh   chan struct{}
	stopped  sync.Once

	mu           sync.RWMutex
	cachedAttest *enclave.AttestationResponse
}

// NewTEEProxy creates a real TEE proxy that communicates with the enclave over vsock.
func NewTEEProxy(cid, port uint32, registry *platform.PositionRegistry, budgets *platform.BudgetTracker) *TEEProxy {
	return &TEEProxy{
		client:   &VsockClient{CID: cid, Port: port},
		registry: registry,
		budgets:  budgets,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background sync goroutine.
// On start: push positions + budgets, fetch attestation.
// Every 5s: push positions + budgets.
// Every 60s: refresh attestation cache.
func (p *TEEProxy) Start() {
	p.syncPositions()
	p.syncBudgets()
	p.refreshAttestation()

	go func() {
		syncTicker := time.NewTicker(5 * time.Second)
		attestTicker := time.NewTicker(60 * time.Second)
		defer syncTicker.Stop()
		defer attestTicker.Stop()

		for {
			select {
			case <-syncTicker.C:
				p.syncPositions()
				p.syncBudgets()
			case <-attestTicker.C:
				p.refreshAttestation()
			case <-p.stopCh:
				return
			}
		}
	}()
}

// Stop shuts down the background sync.
func (p *TEEProxy) Stop() {
	p.stopped.Do(func() { close(p.stopCh) })
}

func (p *TEEProxy) syncPositions() {
	positions := p.registry.GetAll()
	snaps := make([]enclave.PositionSnapshot, len(positions))
	for i, pos := range positions {
		snaps[i] = enclave.PositionSnapshot{
			ID:        pos.ID,
			Name:      pos.Name,
			Embedding: pos.Embedding,
			Sigma:     pos.Sigma,
			BidPrice:  pos.BidPrice,
			Currency:  pos.Currency,
			URL:       pos.URL,
		}
	}

	if _, err := p.client.Send("sync_positions", snaps); err != nil {
		log.Printf("[tee] sync positions failed: %v", err)
	}
}

func (p *TEEProxy) syncBudgets() {
	positions := p.registry.GetAll()
	var snaps []enclave.BudgetSnapshot
	for _, pos := range positions {
		info := p.budgets.GetInfo(pos.ID)
		if info == nil {
			continue
		}
		snaps = append(snaps, enclave.BudgetSnapshot{
			AdvertiserID: pos.ID,
			Total:        info.Total,
			Spent:        info.Spent,
			Currency:     info.Currency,
		})
	}

	if _, err := p.client.Send("sync_budgets", snaps); err != nil {
		log.Printf("[tee] sync budgets failed: %v", err)
	}
}

func (p *TEEProxy) refreshAttestation() {
	resp, err := p.client.Send("key_request", nil)
	if err != nil {
		log.Printf("[tee] refresh attestation failed: %v", err)
		return
	}

	var attest enclave.AttestationResponse
	if err := json.Unmarshal(resp.Payload, &attest); err != nil {
		log.Printf("[tee] unmarshal attestation failed: %v", err)
		return
	}

	p.mu.Lock()
	p.cachedAttest = &attest
	p.mu.Unlock()
	log.Println("[tee] attestation refreshed")
}

func (p *TEEProxy) GetAttestation() (*enclave.AttestationResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cachedAttest != nil {
		return p.cachedAttest, nil
	}
	return nil, fmt.Errorf("attestation not yet available (enclave not connected)")
}

func (p *TEEProxy) RunAuction(req *enclave.AuctionRequest) (*enclave.AuctionResponse, error) {
	resp, err := p.client.Send("auction_request", req)
	if err != nil {
		return nil, err
	}

	var result enclave.AuctionResponse
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("unmarshal auction response: %w", err)
	}
	return &result, nil
}

func (p *TEEProxy) SyncPositions(positions []enclave.PositionSnapshot) error {
	_, err := p.client.Send("sync_positions", positions)
	return err
}

func (p *TEEProxy) SyncBudgets(budgets []enclave.BudgetSnapshot) error {
	_, err := p.client.Send("sync_budgets", budgets)
	return err
}
