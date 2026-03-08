package main

import (
	"bytes"
	"vectorspace/enclave"
	"vectorspace/handler"
	"vectorspace/platform"
	"vectorspace/tee"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	log.SetOutput(io.Discard)
}

// fakeSidecar returns a test HTTP server that produces deterministic embeddings.
func fakeSidecar(embDim int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text  string   `json:"text"`
			Texts []string `json:"texts"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		makeEmb := func(seed int) []float64 {
			emb := make([]float64, embDim)
			for d := range emb {
				emb[d] = float64(seed+1) * 0.01 * float64(d+1)
			}
			return emb
		}

		w.Header().Set("Content-Type", "application/json")
		if req.Texts != nil {
			embeddings := make([][]float64, len(req.Texts))
			for i := range req.Texts {
				embeddings[i] = makeEmb(i)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embeddings": embeddings,
				"dim":        embDim,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embedding": makeEmb(0),
				"dim":       embDim,
			})
		}
	}))
}

func setupServer(numAdvertisers int, embDim int) (http.Handler, *httptest.Server, *tee.MockTEEProxy) {
	sidecar := fakeSidecar(embDim)
	embedder := platform.NewEmbedder(sidecar.URL)
	registry := platform.NewPositionRegistry(embedder)
	budgets := platform.NewBudgetTracker()
	engine := platform.NewAuctionEngine(registry, budgets, embedder)

	proxy, err := tee.NewMockTEEProxy()
	if err != nil {
		panic(fmt.Sprintf("NewMockTEEProxy: %v", err))
	}

	router := handler.NewRouter(handler.RouterConfig{
		Registry: registry,
		Budgets:  budgets,
		Engine:   engine,
		TEEProxy: proxy,
	})

	// Pre-register advertisers and build position/budget snapshots for TEE
	positions := make([]enclave.PositionSnapshot, 0, numAdvertisers)
	budgetSnaps := make([]enclave.BudgetSnapshot, 0, numAdvertisers)

	for i := 0; i < numAdvertisers; i++ {
		name := fmt.Sprintf("adv-%d", i)
		bidPrice := 2.0 + float64(i)*0.1
		body := map[string]interface{}{
			"name":      name,
			"intent":    fmt.Sprintf("test intent for advertiser %d", i),
			"sigma":     0.5,
			"bid_price": bidPrice,
			"budget":    1e9,
			"currency":  "USD",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/advertiser/register", bytes.NewReader(b))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 201 {
			panic(fmt.Sprintf("register failed: %d %s", w.Code, w.Body.String()))
		}

		var result map[string]interface{}
		json.NewDecoder(w.Body).Decode(&result)
		advID := result["id"].(string)

		emb := make([]float64, embDim)
		for d := range emb {
			emb[d] = float64(i+1) * 0.01 * float64(d+1)
		}
		positions = append(positions, enclave.PositionSnapshot{
			ID: advID, Name: name, Embedding: emb, Sigma: 0.5, BidPrice: bidPrice, Currency: "USD",
		})
		budgetSnaps = append(budgetSnaps, enclave.BudgetSnapshot{
			AdvertiserID: advID, Total: 1e9, Spent: 0, Currency: "USD",
		})
	}

	proxy.SyncPositions(positions)
	proxy.SyncBudgets(budgetSnaps)

	return router, sidecar, proxy
}

func makeEncryptedAdRequestBody(proxy *tee.MockTEEProxy, embDim int) []byte {
	queryEmbedding := make([]float64, embDim)
	for d := range queryEmbedding {
		queryEmbedding[d] = 0.01 * float64(d+1)
	}
	embJSON, _ := json.Marshal(queryEmbedding)
	pubKey := proxy.KeyManagerPublicKey()
	aesKeyEnc, payloadEnc, nonce, err := enclave.EncryptHybrid(embJSON, &pubKey, enclave.HashAlgorithmSHA256)
	if err != nil {
		panic(fmt.Sprintf("EncryptHybrid: %v", err))
	}
	body := map[string]interface{}{
		"encrypted_embedding": enclave.EncryptedEmbedding{
			AESKeyEncrypted:  aesKeyEnc,
			EncryptedPayload: payloadEnc,
			Nonce:            nonce,
			HashAlgorithm:    "SHA-256",
		},
	}
	b, _ := json.Marshal(body)
	return b
}

// BenchmarkAdRequest_10adv_3dim — small scenario
func BenchmarkAdRequest_10adv_3dim(b *testing.B) {
	router, sidecar, proxy := setupServer(10, 3)
	defer sidecar.Close()
	body := makeEncryptedAdRequestBody(proxy, 3)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != 200 {
				b.Fatalf("status %d", w.Code)
			}
		}
	})
}

// BenchmarkAdRequest_100adv_384dim — realistic scenario (384-dim embeddings)
func BenchmarkAdRequest_100adv_384dim(b *testing.B) {
	router, sidecar, proxy := setupServer(100, 384)
	defer sidecar.Close()
	body := makeEncryptedAdRequestBody(proxy, 384)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != 200 {
				b.Fatalf("status %d", w.Code)
			}
		}
	})
}

// BenchmarkAdRequest_1000adv_384dim — large scenario
func BenchmarkAdRequest_1000adv_384dim(b *testing.B) {
	router, sidecar, proxy := setupServer(1000, 384)
	defer sidecar.Close()
	body := makeEncryptedAdRequestBody(proxy, 384)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != 200 {
				b.Fatalf("status %d", w.Code)
			}
		}
	})
}

// TestConcurrentThroughput runs a timed concurrent load test and reports QPS
func TestConcurrentThroughput(t *testing.T) {
	scenarios := []struct {
		name           string
		numAdvertisers int
		embDim         int
		concurrency    int
		duration       time.Duration
	}{
		{"10adv_3dim_c50", 10, 3, 50, 3 * time.Second},
		{"100adv_384dim_c50", 100, 384, 50, 3 * time.Second},
		{"100adv_384dim_c200", 100, 384, 200, 3 * time.Second},
		{"1000adv_384dim_c50", 1000, 384, 50, 3 * time.Second},
		{"1000adv_384dim_c200", 1000, 384, 200, 3 * time.Second},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			router, sidecar, proxy := setupServer(sc.numAdvertisers, sc.embDim)
			t.Cleanup(sidecar.Close)
			body := makeEncryptedAdRequestBody(proxy, sc.embDim)

			var total atomic.Int64
			var errors atomic.Int64
			var wg sync.WaitGroup
			stop := make(chan struct{})

			for i := 0; i < sc.concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-stop:
							return
						default:
						}
						req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
						w := httptest.NewRecorder()
						router.ServeHTTP(w, req)
						if w.Code != 200 {
							errors.Add(1)
						}
						total.Add(1)
					}
				}()
			}

			time.Sleep(sc.duration)
			close(stop)
			wg.Wait()

			count := total.Load()
			errs := errors.Load()
			qps := float64(count) / sc.duration.Seconds()
			t.Logf("  %s: %d requests in %s = %.0f QPS (errors: %d, concurrency: %d)",
				sc.name, count, sc.duration, qps, errs, sc.concurrency)
		})
	}
}
