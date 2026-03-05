package main

import (
	"bytes"
	"cloudx-adserver/handler"
	"cloudx-adserver/platform"
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

func setupServer(numAdvertisers int, embDim int) (http.Handler, *httptest.Server) {
	sidecar := fakeSidecar(embDim)
	embedder := platform.NewEmbedder(sidecar.URL)
	registry := platform.NewPositionRegistry(embedder)
	budgets := platform.NewBudgetTracker()
	engine := platform.NewAuctionEngine(registry, budgets, embedder)
	router := handler.NewRouter(handler.RouterConfig{
		Registry: registry,
		Budgets:  budgets,
		Engine:   engine,
	})

	// Pre-register advertisers
	for i := 0; i < numAdvertisers; i++ {
		body := map[string]interface{}{
			"name":      fmt.Sprintf("adv-%d", i),
			"intent":    fmt.Sprintf("test intent for advertiser %d", i),
			"sigma":     0.5,
			"bid_price": 2.0 + float64(i)*0.1,
			"budget":    1e9, // large budget so we don't run out
			"currency":  "USD",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/advertiser/register", bytes.NewReader(b))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 201 {
			panic(fmt.Sprintf("register failed: %d %s", w.Code, w.Body.String()))
		}
	}

	return router, sidecar
}

func makeAdRequestBody() []byte {
	b, _ := json.Marshal(map[string]interface{}{"intent": "test query intent"})
	return b
}

// BenchmarkAdRequest_10adv_3dim — small scenario
func BenchmarkAdRequest_10adv_3dim(b *testing.B) {
	router, sidecar := setupServer(10, 3)
	defer sidecar.Close()
	body := makeAdRequestBody()
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
	router, sidecar := setupServer(100, 384)
	defer sidecar.Close()
	body := makeAdRequestBody()
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
	router, sidecar := setupServer(1000, 384)
	defer sidecar.Close()
	body := makeAdRequestBody()
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
			router, sidecar := setupServer(sc.numAdvertisers, sc.embDim)
			t.Cleanup(sidecar.Close)
			body := makeAdRequestBody()

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
