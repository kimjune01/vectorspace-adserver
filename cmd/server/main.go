package main

import (
	"vectorspace/handler"
	"vectorspace/platform"
	"vectorspace/tee"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// seedAdvertiser holds one advertiser to be seeded.
type seedAdvertiser struct {
	Name     string
	Intent   string
	Sigma    float64
	BidPrice float64
	Budget   float64
	URL      string
	Title    string // creative title
	Subtitle string // creative subtitle
}

// seedAdvertisers is the default roster covering all demo publisher verticals.
var seedAdvertisers = []seedAdvertiser{
	// Mental Health & Therapy
	{"BetterHelp", "Online therapy platform matching individuals with licensed therapists for weekly video sessions covering anxiety, relationships, and emotional regulation", 0.50, 20.00, 3000,
		"https://www.betterhelp.com", "Talk to a Licensed Therapist", "Start online therapy from home — matched in 48 hours"},
	{"Headspace", "Meditation and mindfulness app with guided sessions for stress relief, better sleep, and daily anxiety management", 0.45, 12.00, 2000,
		"https://www.headspace.com", "Calm Your Mind Today", "Guided meditation for stress, sleep, and focus"},
	{"Talkiatry", "Online psychiatry practice offering medication management and therapy for people dealing with anxiety, depression, and adjustment disorders", 0.40, 22.00, 3000,
		"https://www.talkiatry.com", "See a Psychiatrist Online", "Insurance-covered psychiatric care, often within days"},
	{"Cerebral", "Online mental health platform providing therapy and medication management for people ready to address persistent mood and behavioral issues", 0.45, 20.00, 3000,
		"https://www.cerebral.com", "Mental Health Care Online", "Therapy and medication management — plans from $99/mo"},

	// Sleep & Wellness
	{"DriftOff Sleep", "Sleep improvement program combining CBT-I techniques and guided relaxation to help people with insomnia fall asleep naturally", 0.50, 14.00, 2000,
		"https://www.driftoffsleep.com", "Finally Sleep Through the Night", "CBT-I based program — no pills, no gimmicks"},
	{"Calm", "Wellness app offering sleep stories, breathing exercises, and relaxation techniques for people who can't turn their mind off at night", 0.45, 10.00, 2000,
		"https://www.calm.com", "Sleep Better Tonight", "Sleep stories, breathing exercises, and relaxation"},

	// Social & Connection
	{"Bumble BFF", "Friend-finding feature helping people who just moved to a new city meet locals with shared interests and hobbies", 0.45, 8.00, 1500,
		"https://bumble.com/bff", "Make Friends in Your City", "Swipe to find people who share your hobbies"},
	{"Supportiv", "Peer support chat platform connecting people feeling isolated or overwhelmed to moderated small-group conversations in real time", 0.50, 10.00, 1500,
		"https://www.supportiv.com", "Talk to Someone Right Now", "Anonymous peer support — no appointments, no waitlists"},

	// Relationships & Couples
	{"Regain", "Online couples and relationship counseling helping people who are having conflicts with partners, family, or close relationships", 0.50, 18.00, 2500,
		"https://www.regain.us", "Couples Counseling Online", "Work through relationship issues with a licensed therapist"},
	{"Lasting", "Marriage and relationship health app with guided exercises for couples working through communication problems and emotional distance", 0.45, 12.00, 2000,
		"https://www.getlasting.com", "Strengthen Your Relationship", "Science-based couples exercises you do together"},

	// Nutrition & Health
	{"NutriPlan Pro", "Registered dietitian creating personalized meal plans for people managing high cholesterol, heart disease risk, and weight through diet changes", 0.50, 14.00, 2000,
		"https://www.nutriplanpro.com", "Personalized Meal Plans", "RD-crafted plans for cholesterol, heart health, and weight"},
	{"Noom", "Weight and metabolic health program combining dietary coaching with clinical guidance for people trying to avoid medication through lifestyle changes", 0.45, 15.00, 2500,
		"https://www.noom.com", "Healthy Weight, Your Way", "Psychology-based coaching for lasting behavior change"},
	{"Lark Health", "AI health coach helping people lower cholesterol and manage chronic conditions through daily nutrition tracking and behavior change programs", 0.50, 16.00, 2500,
		"https://www.lark.com", "Your AI Health Coach", "24/7 coaching for diabetes prevention and heart health"},

	// Telehealth & Medical
	{"Sesame Care", "Affordable telehealth platform connecting patients directly with doctors for prescriptions, lab reviews, and second opinions without insurance", 0.50, 16.00, 2500,
		"https://www.sesamecare.com", "See a Doctor for $29", "No insurance needed — prescriptions, labs, and second opinions"},
	{"HeartScore", "Cardiovascular risk screening service that analyzes lab results and provides personalized heart health action plans with lifestyle recommendations", 0.40, 18.00, 2500,
		"https://www.heartscore.com", "Know Your Heart Risk", "Upload labs, get a personalized cardiovascular action plan"},

	// Physical Therapy & Recovery
	{"Hinge Health", "Digital physical therapy platform with sensor-guided exercises for people dealing with back pain, knee injuries, and post-surgical recovery", 0.45, 18.00, 2500,
		"https://www.hingehealth.com", "Physical Therapy at Home", "Sensor-guided exercises for back and joint pain"},
	{"Sword Health", "AI-powered physical therapy with motion-tracking sensors helping patients recover from orthopedic injuries and chronic pain without in-person visits", 0.45, 16.00, 2500,
		"https://www.swordhealth.com", "Recovery Guided by AI", "Motion-tracked PT exercises — faster recovery from home"},

	// Women's Health
	{"Maven Clinic", "Virtual clinic for women's and family health covering fertility, pregnancy, postpartum support, and pediatric care with specialist providers", 0.45, 20.00, 3000,
		"https://www.mavenclinic.com", "Women's Health Specialists", "Fertility, pregnancy, and postpartum care online"},
	{"Nurx", "Telehealth platform prescribing birth control, PrEP, migraine treatment, and dermatology medications delivered to your door", 0.50, 14.00, 2000,
		"https://www.nurx.com", "Prescriptions Delivered", "Birth control, migraine meds, and skincare — by mail"},

	// Substance & Addiction
	{"Quit Genius", "Digital addiction treatment platform using CBT and medication-assisted treatment for people trying to quit smoking, vaping, or drinking", 0.40, 20.00, 3000,
		"https://www.quitgenius.com", "Quit for Good This Time", "CBT + medication support for smoking, vaping, and alcohol"},

	// Chronic Conditions
	{"Omada Health", "Chronic condition management platform for people with prediabetes, hypertension, or obesity combining coaching, devices, and behavioral science", 0.45, 18.00, 2500,
		"https://www.omadahealth.com", "Prevent Diabetes, Naturally", "Connected devices + coaching for lasting health changes"},
	{"Virta Health", "Diabetes reversal program using nutritional ketosis and physician-supervised care to reduce or eliminate diabetes medication", 0.40, 22.00, 3000,
		"https://www.virtahealth.com", "Reverse Type 2 Diabetes", "Physician-supervised nutrition program — medication reduction"},

	// Developer Tools (keep a few for vertical diversity)
	{"Datadog", "Observability platform helping engineering teams monitor infrastructure, detect outages, trace bottlenecks, and debug production incidents", 0.50, 18.00, 3000,
		"https://www.datadoghq.com", "Monitor Everything", "Infrastructure monitoring, APM, and log management"},
	{"PlanetScale", "Managed database service with automatic connection pooling, branching, and horizontal scaling for teams hitting capacity limits", 0.45, 20.00, 3000,
		"https://www.planetscale.com", "Scale Your Database", "Serverless MySQL with branching and zero-downtime deploys"},

	// Tax & Bookkeeping (keep a couple)
	{"GigTax Pro", "Tax preparation service built for gig workers and 1099 contractors to track income, maximize deductions, and file quarterly estimated taxes", 0.50, 16.00, 2500,
		"https://www.gigtaxpro.com", "Taxes for Gig Workers", "Maximize deductions, file quarterlies, stay compliant"},
	{"Collective", "All-in-one back office for self-employed people handling bookkeeping, tax filing, and compliance so freelancers don't miss deductions", 0.50, 18.00, 2500,
		"https://www.collective.com", "Your Back Office, Handled", "Bookkeeping, tax filing, and compliance in one place"},

	// Insurance
	{"Stride Health", "Health insurance marketplace helping gig workers and freelancers find affordable coverage and track tax-deductible premiums", 0.45, 14.00, 2500,
		"https://www.stridehealth.com", "Health Insurance for You", "Find affordable plans and track deductible premiums"},

	// Legal
	{"Trust & Will", "Online estate planning platform helping people create wills, trusts, and beneficiary designations without expensive attorney fees", 0.50, 16.00, 3000,
		"https://trustandwill.com", "Create Your Will Online", "Estate planning in minutes — no attorney fees"},

	// Education
	{"Wyzant", "Online tutoring marketplace connecting students with expert tutors for one-on-one help with difficult college courses and exam preparation", 0.50, 12.00, 2000,
		"https://www.wyzant.com", "Find Your Perfect Tutor", "1-on-1 expert tutoring for any subject"},
	{"Kaplan", "Test preparation service offering structured study plans, practice exams, and expert instruction for students facing high-stakes academic tests", 0.45, 14.00, 2000,
		"https://www.kaptest.com", "Ace Your Next Exam", "Structured study plans, practice tests, expert instruction"},
}

// healthQueries are realistic user intents that a health chatbot would surface.
// Each maps to one or more seed advertisers.
var healthQueries = []string{
	"I've been feeling anxious and can't sleep at night",
	"My doctor said my cholesterol is too high",
	"I need to talk to someone about my depression",
	"My knee hurts after running and I can't exercise",
	"I want to lose weight but diets never work for me",
	"I'm trying to quit smoking but I keep relapsing",
	"My back pain is making it hard to work",
	"I feel lonely since moving to a new city",
	"I just found out I'm pre-diabetic",
	"My partner and I keep fighting about everything",
	"I can't stop worrying about my heart health",
	"I need a therapist but can't afford one",
	"I'm pregnant and have questions about prenatal care",
	"I haven't been sleeping well for months",
	"My stress levels are through the roof at work",
	"I need help managing my diabetes medication",
	"I want to meditate but don't know where to start",
	"I'm dealing with grief after losing a parent",
	"My blood pressure has been high lately",
	"I need to find a doctor but don't have insurance",
	"I'm struggling with postpartum anxiety",
	"I drink too much and want to cut back",
	"I need physical therapy but can't make it to appointments",
	"My teenager is struggling in school and needs a tutor",
	"I need to create a will but lawyers are expensive",
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
	hfToken := flag.String("hf-token", "", "Hugging Face API token (uses HF Inference API instead of sidecar)")
	hfModel := flag.String("hf-model", "BAAI/bge-small-en-v1.5", "Hugging Face embedding model")
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

	if *hfToken == "" {
		*hfToken = os.Getenv("HF_TOKEN")
	}

	var embedder *platform.Embedder
	if *hfToken != "" {
		embedder = platform.NewHuggingFaceEmbedder(*hfModel, *hfToken)
		log.Printf("Using Hugging Face Inference API (model: %s)", *hfModel)
	} else {
		embedder = platform.NewEmbedder(*sidecarURL)
	}
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
				pos, err := registry.RegisterWithBudget(s.Name, s.Intent, s.Sigma, s.BidPrice, s.Budget, "USD", s.URL)
				if err != nil {
					log.Printf("WARN: failed to seed %s: %v", s.Name, err)
					continue
				}
				budgets.Set(pos.ID, s.Budget, "USD")
				// Seed creative if provided
				if s.Title != "" {
					db.InsertCreative(pos.ID, s.Title, s.Subtitle)
				}
				log.Printf("  Seeded: %s (id=%s, sigma=%.2f, bid=$%.2f)", s.Name, pos.ID, s.Sigma, s.BidPrice)
			}
			log.Println("Seeding complete.")

			// Seed health publishers + auction history
			seedHealthData(db, engine, budgets)
		} else {
			log.Printf("Database already has %d advertisers, skipping seed.", len(existing))
		}
	}

	// TEE proxy setup — TEE is always required (default to mock for local dev)
	var teeProxy tee.TEEProxyInterface
	if *teeEnabled {
		realProxy := tee.NewTEEProxy(uint32(*teeCID), uint32(*teePort), registry, budgets)
		realProxy.Start()
		defer realProxy.Stop()
		teeProxy = realProxy
		log.Printf("TEE mode enabled (CID=%d, port=%d)", *teeCID, *teePort)
	} else {
		mockProxy, err := tee.NewMockTEEProxy()
		if err != nil {
			log.Fatalf("Failed to create mock TEE proxy: %v", err)
		}
		tee.SyncFromPlatform(mockProxy, registry, budgets)
		teeProxy = mockProxy
		if *teeMock {
			log.Println("TEE mock mode enabled (in-process auction)")
		} else {
			log.Println("TEE mock mode (default — use --tee for real enclave)")
		}
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

	if *hfToken != "" {
		log.Printf("CloudX Ad Server starting on :8080 (embeddings: Hugging Face)")
	} else {
		log.Printf("CloudX Ad Server starting on :8080 (sidecar: %s)", *sidecarURL)
	}
	if *anthropicKey != "" {
		log.Println("Chat proxy enabled (Anthropic API key configured)")
	}
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}

// seedHealthData creates demo publishers, generates mock auction/event history,
// and seeds tokens so researchers can immediately open dashboard URLs.
func seedHealthData(db *platform.DB, engine *platform.AuctionEngine, budgets *platform.BudgetTracker) {
	log.Println("Seeding health demo data (publishers, auctions, events)...")

	// Create two health publishers
	publishers := []struct {
		ID       string
		Name     string
		Domain   string
		Email    string
		Password string
	}{
		{"pub-1", "HealthChat AI", "healthchat.ai", "demo@healthchat.ai", "demo1234"},
		{"pub-2", "MindfulBot", "mindfulbot.com", "demo@mindfulbot.com", "demo1234"},
	}

	for _, p := range publishers {
		if err := db.InsertPublisher(p.ID, p.Name, p.Domain); err != nil {
			log.Printf("WARN: failed to seed publisher %s: %v", p.Name, err)
			continue
		}
		token, err := db.GeneratePublisherToken(p.ID)
		if err != nil {
			log.Printf("WARN: failed to generate token for %s: %v", p.Name, err)
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("WARN: failed to hash password for %s: %v", p.Name, err)
			continue
		}
		db.InsertPublisherCredentials(p.ID, p.Email, string(hash))
		log.Printf("  Publisher: %s (token=%s, login=%s / %s)", p.Name, token, p.Email, p.Password)
	}

	// Generate advertiser tokens so portal links work
	allPositions := engine.Registry.GetAll()
	for _, pos := range allPositions {
		token, err := db.GenerateToken(pos.ID)
		if err != nil {
			continue
		}
		log.Printf("  Advertiser token: %s → %s", pos.ID, token)
	}

	// Seed auction + event history over the last 14 days
	rng := rand.New(rand.NewSource(42)) // deterministic for reproducibility
	now := time.Now()
	userIDs := []string{"user-a1b2", "user-c3d4", "user-e5f6", "user-g7h8", "user-i9j0",
		"user-k1l2", "user-m3n4", "user-o5p6", "user-q7r8", "user-s9t0"}

	auctionCount := 0
	for day := 13; day >= 0; day-- {
		// More auctions on recent days (ramp up)
		dailyAuctions := 4 + rng.Intn(6) + (14-day)/2
		for i := 0; i < dailyAuctions; i++ {
			// Pick a random health query
			query := healthQueries[rng.Intn(len(healthQueries))]

			// Run a real auction to get realistic winner + payment
			result, err := engine.SimulateAuction(query, 0)
			if err != nil {
				continue
			}

			// Pick publisher
			pubID := publishers[rng.Intn(len(publishers))].ID

			// Log auction with backdated timestamp
			hoursOffset := rng.Intn(24)
			auctionTime := now.AddDate(0, 0, -day).Add(-time.Duration(hoursOffset) * time.Hour)
			auctionID, err := logBackdatedAuction(db, query, result.Winner.ID, result.Payment, "USD", result.BidCount, pubID, auctionTime)
			if err != nil {
				continue
			}

			// Always log impression
			userID := userIDs[rng.Intn(len(userIDs))]
			logBackdatedEvent(db, auctionID, result.Winner.ID, "impression", userID, pubID, auctionTime.Add(time.Duration(rng.Intn(5))*time.Second))

			// 70% chance of viewable
			if rng.Float64() < 0.70 {
				logBackdatedEvent(db, auctionID, result.Winner.ID, "viewable", userID, pubID, auctionTime.Add(time.Duration(1+rng.Intn(3))*time.Second))
			}

			// 15% chance of click (charge budget)
			if rng.Float64() < 0.15 {
				logBackdatedEvent(db, auctionID, result.Winner.ID, "click", userID, pubID, auctionTime.Add(time.Duration(2+rng.Intn(10))*time.Second))
				budgets.Charge(result.Winner.ID, result.Payment)
			}

			auctionCount++
		}
	}

	log.Printf("  Seeded %d auctions with events over 14 days", auctionCount)
	log.Println("Health demo data seeding complete.")
	log.Println()
	log.Println("=== DEMO ACCESS ===")
	log.Println("  Admin:     http://localhost:8080/admin  (use --admin-password)")
	log.Printf("  Publisher login: demo@healthchat.ai / demo1234")
	log.Printf("  Publisher login: demo@mindfulbot.com / demo1234")
	log.Println("===================")
}

// logBackdatedAuction inserts an auction with a specific timestamp.
func logBackdatedAuction(db *platform.DB, intent, winnerID string, payment float64, currency string, bidCount int, publisherID string, ts time.Time) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO auctions (intent, winner_id, payment, currency, bid_count, publisher_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		intent, winnerID, payment, currency, bidCount, publisherID, ts.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// logBackdatedEvent inserts an event with a specific timestamp.
func logBackdatedEvent(db *platform.DB, auctionID int64, advertiserID, eventType, userID, publisherID string, ts time.Time) {
	db.Exec(
		`INSERT INTO events (auction_id, advertiser_id, event_type, user_id, publisher_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		auctionID, advertiserID, eventType, userID, publisherID, ts.Format("2006-01-02 15:04:05"),
	)
}
