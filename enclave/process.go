package enclave

import (
	"vectorspace/enclave/auction"
	"crypto/rsa"
	"encoding/json"
	"fmt"
)

// ProcessPrivateAuction decrypts the embedding, runs the auction with live
// budgets/pacing, and returns the result. The embedding is zeroed after use.
func ProcessPrivateAuction(
	req *AuctionRequest,
	privateKey *rsa.PrivateKey,
	positions *PositionStore,
	budgets *BudgetStore,
) (*AuctionResponse, error) {
	// 1. Get embedding: use plain vector if provided, otherwise decrypt
	var queryEmbedding []float64

	if len(req.Embedding) > 0 {
		// Plain embedding path — copy to avoid mutating the request
		queryEmbedding = make([]float64, len(req.Embedding))
		copy(queryEmbedding, req.Embedding)
	} else {
		hashAlg := HashAlgorithm(req.EncryptedEmbedding.HashAlgorithm)
		if hashAlg == "" {
			hashAlg = HashAlgorithmSHA256
		}

		plaintext, err := DecryptHybrid(
			req.EncryptedEmbedding.AESKeyEncrypted,
			req.EncryptedEmbedding.EncryptedPayload,
			req.EncryptedEmbedding.Nonce,
			privateKey,
			hashAlg,
		)
		if err != nil {
			return nil, fmt.Errorf("decrypt embedding: %w", err)
		}

		if err := json.Unmarshal(plaintext, &queryEmbedding); err != nil {
			return nil, fmt.Errorf("unmarshal embedding: %w", err)
		}
	}

	// Ensure we zero the embedding when done
	defer func() {
		for i := range queryEmbedding {
			queryEmbedding[i] = 0
		}
	}()

	// 2. Load positions
	allPositions := positions.GetAll()
	if len(allPositions) == 0 {
		return nil, fmt.Errorf("no registered advertisers")
	}

	// 3. Build bids from positions that can afford their bid price
	var bids []auction.CoreBid
	for _, pos := range allPositions {
		if !budgets.CanAfford(pos.ID, pos.BidPrice) {
			continue
		}
		bids = append(bids, auction.CoreBid{
			ID:        pos.ID,
			Bidder:    pos.Name,
			Price:     pos.BidPrice,
			Currency:  pos.Currency,
			Embedding: pos.Embedding,
			Sigma:     pos.Sigma,
		})
	}

	if len(bids) == 0 {
		return nil, fmt.Errorf("no eligible bidders (all out of budget)")
	}

	// 4. Apply tau filter
	if req.Tau > 0 {
		var filtered []auction.CoreBid
		for _, bid := range bids {
			distSq := auction.SquaredEuclideanDistance(bid.Embedding, queryEmbedding)
			if distSq <= req.Tau {
				filtered = append(filtered, bid)
			}
		}
		bids = filtered
		if len(bids) == 0 {
			return nil, fmt.Errorf("no bidders passed relevance threshold (tau=%.4f)", req.Tau)
		}
	}

	bidCount := len(bids)

	// 5. Run auction (use publisher's log base if set, otherwise default)
	logBase := req.LogBase
	if logBase <= 0 {
		logBase = auction.DefaultLogBase
	}
	result := auction.RunAuctionWithBase(bids, 0, logBase, queryEmbedding)
	if result.Winner == nil {
		return nil, fmt.Errorf("auction produced no winner")
	}

	// 6. Compute VCG payment
	payment := auction.ComputeVCGPaymentWithBase(result, queryEmbedding, logBase)

	return &AuctionResponse{
		WinnerID: result.Winner.ID,
		Payment:  payment,
		Currency: result.Winner.Currency,
		BidCount: bidCount,
	}, nil
}
