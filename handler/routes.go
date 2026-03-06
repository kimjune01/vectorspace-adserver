package handler

import (
	"cloudx-adserver/platform"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// loggingMiddleware logs each request's method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

// corsMiddleware adds CORS headers for the frontend dev server.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// RouterConfig contains all dependencies for the router.
type RouterConfig struct {
	Registry     *platform.PositionRegistry
	Budgets      *platform.BudgetTracker
	Engine       *platform.AuctionEngine
	DB           *platform.DB
	AnthropicKey string
}

// NewRouter wires all routes and returns an http.Handler.
func NewRouter(cfg RouterConfig) http.Handler {
	advHandler := &AdvertiserHandler{Registry: cfg.Registry, Budgets: cfg.Budgets, DB: cfg.DB}
	pubHandler := &PublisherHandler{Engine: cfg.Engine}
	chatHandler := &ChatHandler{APIKey: cfg.AnthropicKey}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/advertiser/register", advHandler.HandleRegister)
	mux.HandleFunc("/advertiser/", advHandler.HandleAdvertiser)
	mux.HandleFunc("/positions", advHandler.HandlePositions)
	mux.HandleFunc("/budget/", advHandler.HandleBudget)
	mux.HandleFunc("/ad-request", pubHandler.HandleAdRequest)
	mux.HandleFunc("/chat", chatHandler.HandleChat)

	if cfg.DB != nil {
		db := cfg.DB
		mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				stats, err := db.GetStats()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(stats)
			case http.MethodDelete:
				if err := db.ResetStats(); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
	}

	return corsMiddleware(loggingMiddleware(mux))
}
