package main

import (
	"cloudx-adserver/handler"
	"cloudx-adserver/platform"
	"cloudx-adserver/tee"
	"flag"
	"log"
	"net/http"
	"os"
)

// seedAdvertisers is the default roster covering all demo publisher verticals.
var seedAdvertisers = []struct {
	Name     string
	Intent   string
	Sigma    float64
	BidPrice float64
	Budget   float64
}{
	// Mental Health & Therapy
	{"BetterHelp", "Online therapy platform matching individuals with licensed therapists for weekly video sessions covering anxiety, relationships, and emotional regulation", 0.50, 20.00, 3000},
	{"Headspace", "Meditation and mindfulness app with guided sessions for stress relief, better sleep, and daily anxiety management", 0.45, 12.00, 2000},
	{"Talkiatry", "Online psychiatry practice offering medication management and therapy for people dealing with anxiety, depression, and adjustment disorders", 0.40, 22.00, 3000},
	{"Cerebral", "Online mental health platform providing therapy and medication management for people ready to address persistent mood and behavioral issues", 0.45, 20.00, 3000},

	// Sleep & Wellness
	{"DriftOff Sleep", "Sleep improvement program combining CBT-I techniques and guided relaxation to help people with insomnia fall asleep naturally", 0.50, 14.00, 2000},
	{"Calm", "Wellness app offering sleep stories, breathing exercises, and relaxation techniques for people who can't turn their mind off at night", 0.45, 10.00, 2000},

	// Social & Connection
	{"Bumble BFF", "Friend-finding feature helping people who just moved to a new city meet locals with shared interests and hobbies", 0.80, 8.00, 1500},
	{"Supportiv", "Peer support chat platform connecting people feeling isolated or overwhelmed to moderated small-group conversations in real time", 0.50, 10.00, 1500},

	// Relationships & Couples
	{"Regain", "Online couples and relationship counseling helping people who are having conflicts with partners, family, or close relationships", 0.50, 18.00, 2500},
	{"Lasting", "Marriage and relationship health app with guided exercises for couples working through communication problems and emotional distance", 0.45, 12.00, 2000},

	// Nutrition & Health
	{"NutriPlan Pro", "Registered dietitian creating personalized meal plans for people managing high cholesterol, heart disease risk, and weight through diet changes", 0.50, 14.00, 2000},
	{"Noom", "Weight and metabolic health program combining dietary coaching with clinical guidance for people trying to avoid medication through lifestyle changes", 0.45, 15.00, 2500},
	{"Lark Health", "AI health coach helping people lower cholesterol and manage chronic conditions through daily nutrition tracking and behavior change programs", 0.50, 16.00, 2500},

	// Telehealth & Medical
	{"Sesame Care", "Affordable telehealth platform connecting patients directly with doctors for prescriptions, lab reviews, and second opinions without insurance", 0.50, 16.00, 2500},
	{"HeartScore", "Cardiovascular risk screening service that analyzes lab results and provides personalized heart health action plans with lifestyle recommendations", 0.40, 18.00, 2500},

	// Developer Tools
	{"Datadog", "Observability platform helping engineering teams monitor infrastructure, detect outages, trace bottlenecks, and debug production incidents", 0.50, 18.00, 3000},
	{"PlanetScale", "Managed database service with automatic connection pooling, branching, and horizontal scaling for teams hitting capacity limits", 0.45, 20.00, 3000},
	{"Render", "Cloud platform providing zero-config deployment, autoscaling, and infrastructure management so developers ship without DevOps overhead", 0.50, 16.00, 3000},
	{"LinearB", "Engineering intelligence platform helping teams identify deployment bottlenecks, reduce cycle time, and improve delivery velocity", 0.40, 14.00, 2500},

	// Tax & Bookkeeping
	{"GigTax Pro", "Tax preparation service built for gig workers and 1099 contractors to track income, maximize deductions, and file quarterly estimated taxes", 0.50, 16.00, 2500},
	{"Collective", "All-in-one back office for self-employed people handling bookkeeping, tax filing, and compliance so freelancers don't miss deductions", 0.50, 18.00, 2500},
	{"Hurdlr", "Expense tracking app that automatically logs mileage, receipts, and business expenses for freelancers and side hustlers", 0.45, 12.00, 2000},

	// Insurance
	{"Stride Health", "Health insurance marketplace helping gig workers and freelancers find affordable coverage and track tax-deductible premiums", 0.45, 14.00, 2500},
	{"Haven Life", "Term life insurance provider helping people protect their family's financial future with affordable, no-exam coverage", 0.45, 14.00, 2500},

	// Estate & Legal
	{"Trust & Will", "Online estate planning platform helping people create wills, trusts, and beneficiary designations without expensive attorney fees", 0.50, 16.00, 3000},
	{"EstateGuard", "Estate planning attorney helping families set up trusts, manage inheritance, and minimize estate taxes after a loved one passes", 0.50, 22.00, 4000},
	{"TaxSmith CPA", "CPA firm specializing in inheritance tax, estate settlements, and tax-efficient wealth transfer for families receiving a sudden windfall", 0.45, 18.00, 3000},
	{"AI Lawyer", "AI legal assistant helping people draft contracts, review agreements, conduct legal research, and summarize legal documents without expensive attorney fees", 0.50, 16.00, 3000},

	// Tutoring & Education
	{"Wyzant", "Online tutoring marketplace connecting students with expert tutors for one-on-one help with difficult college courses and exam preparation", 0.50, 12.00, 2000},
	{"Varsity Tutors", "Live online tutoring connecting struggling students with subject-matter experts for personalized help in chemistry, biology, and other STEM courses", 0.50, 14.00, 2000},
	{"Chegg Study", "On-demand tutoring service providing step-by-step homework help and live explanations for students stuck on course material", 0.50, 10.00, 1500},
	{"Kaplan", "Test preparation service offering structured study plans, practice exams, and expert instruction for students facing high-stakes academic tests", 0.45, 14.00, 2000},
	{"Brainscape", "Spaced-repetition flashcard app helping students master difficult subjects through adaptive daily review optimized for long-term retention", 0.45, 8.00, 1500},
}

func main() {
	sidecarURL := flag.String("sidecar-url", "http://localhost:8081", "URL of the embedding sidecar")
	dbPath := flag.String("db-path", "", "Path to SQLite database (empty = in-memory only)")
	seed := flag.Bool("seed", false, "Seed default advertisers on startup (only if DB is empty)")
	anthropicKey := flag.String("anthropic-key", "", "Anthropic API key for /chat proxy")
	freqCapMax := flag.Int("freq-cap-max", 3, "Max impressions per advertiser per user per window")
	freqCapWindow := flag.Int("freq-cap-window", 60, "Frequency cap window in minutes")
	adminPassword := flag.String("admin-password", "", "Password for admin endpoints (empty = no auth)")
	teeEnabled := flag.Bool("tee", false, "Enable real TEE mode (requires Nitro Enclave running)")
	teeMock := flag.Bool("tee-mock", false, "Enable mock TEE mode (local dev, no EC2 needed)")
	teeCID := flag.Int("tee-cid", 16, "Enclave CID for vsock")
	teePort := flag.Int("tee-port", 5000, "Enclave vsock port")
	flag.Parse()

	// Try env var for API key if flag not set
	if *anthropicKey == "" {
		*anthropicKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	// Try env var for admin password if flag not set
	if *adminPassword == "" {
		*adminPassword = os.Getenv("ADMIN_PASSWORD")
	}
	if *adminPassword == "" {
		log.Println("WARNING: --admin-password not set, admin endpoints are unprotected")
	}

	embedder := platform.NewEmbedder(*sidecarURL)
	registry := platform.NewPositionRegistry(embedder)
	budgets := platform.NewBudgetTracker()

	var db *platform.DB

	if *dbPath != "" {
		var err error
		db, err = platform.NewDB(*dbPath)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		if err := registry.SetDB(db); err != nil {
			log.Fatalf("Failed to load registry from DB: %v", err)
		}
		if err := budgets.SetDB(db); err != nil {
			log.Fatalf("Failed to load budgets from DB: %v", err)
		}

		log.Printf("SQLite database: %s", *dbPath)
	}

	engine := platform.NewAuctionEngine(registry, budgets, embedder)
	engine.DB = db

	// Seed default advertisers if requested and DB is empty
	if *seed && db != nil {
		existing := registry.GetAll()
		if len(existing) == 0 {
			log.Printf("Seeding %d default advertisers...", len(seedAdvertisers))
			for _, s := range seedAdvertisers {
				pos, err := registry.RegisterWithBudget(s.Name, s.Intent, s.Sigma, s.BidPrice, s.Budget, "USD", "")
				if err != nil {
					log.Printf("WARN: failed to seed %s: %v", s.Name, err)
					continue
				}
				budgets.Set(pos.ID, s.Budget, "USD")
				log.Printf("  Seeded: %s (id=%s, sigma=%.2f, bid=$%.2f)", s.Name, pos.ID, s.Sigma, s.BidPrice)
			}
			log.Println("Seeding complete.")
		} else {
			log.Printf("Database already has %d advertisers, skipping seed.", len(existing))
		}
	}

	// TEE proxy setup
	var teeProxy tee.TEEProxyInterface
	if *teeMock {
		mockProxy, err := tee.NewMockTEEProxy()
		if err != nil {
			log.Fatalf("Failed to create mock TEE proxy: %v", err)
		}
		tee.SyncFromPlatform(mockProxy, registry, budgets)
		teeProxy = mockProxy
		log.Println("TEE mock mode enabled (in-process auction)")
	} else if *teeEnabled {
		realProxy := tee.NewTEEProxy(uint32(*teeCID), uint32(*teePort), registry, budgets)
		realProxy.Start()
		defer realProxy.Stop()
		teeProxy = realProxy
		log.Printf("TEE mode enabled (CID=%d, port=%d)", *teeCID, *teePort)
	}

	router := handler.NewRouter(handler.RouterConfig{
		Registry:      registry,
		Budgets:       budgets,
		Engine:        engine,
		DB:            db,
		AnthropicKey:  *anthropicKey,
		FreqCapMax:    *freqCapMax,
		FreqCapWindow: *freqCapWindow,
		AdminPassword: *adminPassword,
		TEEProxy:      teeProxy,
	})

	log.Printf("CloudX Ad Server starting on :8080 (sidecar: %s)", *sidecarURL)
	if *anthropicKey != "" {
		log.Println("Chat proxy enabled (Anthropic API key configured)")
	}
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
