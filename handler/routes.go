package handler

import (
	"vectorspace/platform"
	"vectorspace/tee"
	"vectorspace/trust"
	"encoding/json"
	"log"
	"net/http"
	"os"
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Password")

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
	Registry      *platform.PositionRegistry
	Budgets       *platform.BudgetTracker
	Engine        *platform.AuctionEngine
	DB            *platform.DB
	AnthropicKey  string
	FreqCapMax    int
	FreqCapWindow int
	AdminPassword string
	TEEProxy      tee.TEEProxyInterface
	GitHash       string
	TrustLedger   *trust.Ledger
}

// adminAuthMiddleware checks the X-Admin-Password header. If password is empty, all requests pass through (dev mode).
func adminAuthMiddleware(password string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if password != "" && r.Header.Get("X-Admin-Password") != password {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// NewRouter wires all routes and returns an http.Handler.
func NewRouter(cfg RouterConfig) http.Handler {
	advHandler := &AdvertiserHandler{Registry: cfg.Registry, Budgets: cfg.Budgets, DB: cfg.DB}
	pubHandler := &PublisherHandler{Engine: cfg.Engine, DB: cfg.DB}
	chatHandler := &ChatHandler{APIKey: cfg.AnthropicKey}

	freqCapMax := cfg.FreqCapMax
	if freqCapMax <= 0 {
		freqCapMax = 3
	}
	freqCapWindow := cfg.FreqCapWindow
	if freqCapWindow <= 0 {
		freqCapWindow = 60
	}

	mux := http.NewServeMux()

	gitHash := cfg.GitHash
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "gitHash": gitHash})
	})

	mux.HandleFunc("/advertiser/register", advHandler.HandleRegister)
	mux.HandleFunc("/advertiser/", advHandler.HandleAdvertiser)
	mux.HandleFunc("/positions", advHandler.HandlePositions)
	mux.HandleFunc("/budget/", advHandler.HandleBudget)
	mux.HandleFunc("/embeddings", pubHandler.HandleEmbeddings)
	mux.HandleFunc("/embed", pubHandler.HandleEmbed)
	mux.HandleFunc("/simulate", adminAuthMiddleware(cfg.AdminPassword, pubHandler.HandleSimulate))
	mux.HandleFunc("/chat", chatHandler.HandleChat)

	// Publisher registration (admin-protected)
	if cfg.DB != nil {
		mux.HandleFunc("/publisher/register", adminAuthMiddleware(cfg.AdminPassword, pubHandler.HandleRegisterPublisher))
	}

	// Event tracking endpoints
	if cfg.DB != nil {
		eventHandler := &EventHandler{DB: cfg.DB, Budgets: cfg.Budgets, FreqCapMax: freqCapMax, FreqCapWindow: freqCapWindow}
		mux.HandleFunc("/event/impression", eventHandler.HandleImpression)
		mux.HandleFunc("/event/click", eventHandler.HandleClick)
		mux.HandleFunc("/event/viewable", eventHandler.HandleViewable)

		// Portal endpoints (token-authenticated)
		portalHandler := &PortalHandler{Registry: cfg.Registry, Budgets: cfg.Budgets, DB: cfg.DB}
		mux.HandleFunc("/portal/me", portalHandler.HandlePortalMe)
		mux.HandleFunc("/portal/me/auctions", portalHandler.HandlePortalAuctions)
		mux.HandleFunc("/portal/me/events", portalHandler.HandlePortalEvents)
		mux.HandleFunc("/portal/me/creatives", portalHandler.HandlePortalCreatives)
		mux.HandleFunc("/portal/me/creatives/", portalHandler.HandlePortalCreative)

		// Publisher portal endpoints
		pubPortalHandler := &PublisherPortalHandler{DB: cfg.DB}
		mux.HandleFunc("/portal/publisher/me", pubPortalHandler.HandlePublisherMe)
		mux.HandleFunc("/portal/publisher/stats", pubPortalHandler.HandlePublisherStats)
		mux.HandleFunc("/portal/publisher/revenue", pubPortalHandler.HandlePublisherRevenue)
		mux.HandleFunc("/portal/publisher/events", pubPortalHandler.HandlePublisherEvents)
		mux.HandleFunc("/portal/publisher/auctions", pubPortalHandler.HandlePublisherAuctions)
		mux.HandleFunc("/portal/publisher/top-advertisers", pubPortalHandler.HandlePublisherTopAdvertisers)

		// Admin endpoints (password-protected)
		mux.HandleFunc("/admin/auctions", adminAuthMiddleware(cfg.AdminPassword, portalHandler.HandleAdminAuctions))
		mux.HandleFunc("/admin/revenue", adminAuthMiddleware(cfg.AdminPassword, portalHandler.HandleAdminRevenue))
		mux.HandleFunc("/admin/top-advertisers", adminAuthMiddleware(cfg.AdminPassword, portalHandler.HandleAdminTopAdvertisers))
		mux.HandleFunc("/admin/advertisers", adminAuthMiddleware(cfg.AdminPassword, portalHandler.HandleAdminAdvertisers))
		mux.HandleFunc("/admin/events", adminAuthMiddleware(cfg.AdminPassword, portalHandler.HandleAdminEvents))
		mux.HandleFunc("/admin/publishers", adminAuthMiddleware(cfg.AdminPassword, func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				portalHandler.HandleAdminPublishers(w, r)
			case http.MethodPost:
				pubHandler.HandleCreatePublisherWithCredentials(w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		}))

		// Publisher login (public)
		mux.HandleFunc("/publisher/login", pubHandler.HandlePublisherLogin)

		// Intake form submissions (public POST, admin-protected GET)
		intakeHandler := &IntakeHandler{DB: cfg.DB}
		mux.HandleFunc("/intake", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				adminAuthMiddleware(cfg.AdminPassword, intakeHandler.HandleSubmit)(w, r)
				return
			}
			intakeHandler.HandleSubmit(w, r)
		})
	}

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

	// Trust graph / attestation exchange endpoints
	if cfg.TrustLedger != nil {
		trustHandler := &TrustHandler{Ledger: cfg.TrustLedger}
		// Public read endpoints (curators sync these)
		mux.HandleFunc("/trust/graph", trustHandler.HandleGraph)
		mux.HandleFunc("/trust/node/", trustHandler.HandleNode)
		mux.HandleFunc("/trust/attestation/", trustHandler.HandleAttestation)
		mux.HandleFunc("/trust/log", trustHandler.HandleLedgerLog)
		mux.HandleFunc("/trust/allowlist", trustHandler.HandleTrustedAddrs)
		// Write endpoints (admin-only — production attestations arrive via DKIM-signed email)
		mux.HandleFunc("/trust/attest", adminAuthMiddleware(cfg.AdminPassword, trustHandler.HandleSubmitAttestation))
		mux.HandleFunc("/trust/publish", adminAuthMiddleware(cfg.AdminPassword, trustHandler.HandlePublishPreference))
	}

	// TEE endpoints — all auctions run through the enclave
	teeHandler := &TEEHandler{Proxy: cfg.TEEProxy, DB: cfg.DB, Engine: cfg.Engine}
	mux.HandleFunc("/tee/attestation", teeHandler.HandleAttestation)
	mux.HandleFunc("/ad-request", teeHandler.HandleAdRequestPrivate)

	// Serve portal SPA from portal-dist/ as a catch-all fallback.
	// ServeMux matches more-specific routes first, so all API routes take priority.
	if info, err := os.Stat("portal-dist"); err == nil && info.IsDir() {
		spa := http.FileServer(http.Dir("portal-dist"))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the exact file; fall back to index.html for SPA client-side routing.
			path := "portal-dist" + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, "portal-dist/index.html")
				return
			}
			spa.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(loggingMiddleware(mux))
}
