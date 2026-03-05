package main

import (
	"cloudx-adserver/handler"
	"cloudx-adserver/platform"
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
	// Physical Therapy
	{"Peak PT", "Physical therapist helping runners and endurance athletes recover from sports injuries through targeted rehab programs", 0.50, 2.50, 500},
	{"AllMotion PT", "Physical therapist treating back pain, posture problems, and general musculoskeletal issues through movement therapy and hands-on rehab", 0.80, 2.00, 500},
	{"ClimbStrong Rehab", "Physical therapist specializing in finger, hand, and upper extremity injuries common among rock climbers and bouldering athletes", 0.40, 3.00, 500},
	{"NeuroMove PT", "Physical therapist helping stroke survivors and traumatic brain injury patients regain movement through neurological rehabilitation", 0.45, 2.75, 500},

	// Health & Medical (for CodyMD, Doctronic, Counsel Health, August AI demos)
	{"QuickClinic", "Online doctor providing telehealth visits, prescriptions, and urgent care consultations for patients who need medical attention fast", 0.80, 3.50, 600},
	{"MindBridge Therapy", "Licensed therapist offering CBT and counseling sessions to adults struggling with anxiety, depression, and insomnia", 0.50, 4.00, 600},
	{"DermCheck AI", "Dermatology screening tool that helps people identify skin rashes, suspicious moles, and other skin conditions from photos", 0.40, 3.75, 600},
	{"NutriPlan Pro", "Registered dietitian creating personalized meal plans and nutrition guidance for people managing cholesterol, diabetes, or weight loss", 0.45, 2.25, 500},

	// Tutoring & Education (for Brainly demo)
	{"BrightMinds Tutoring", "Academic tutor helping K-12 students improve grades in math, science, reading, and other school subjects through one-on-one lessons", 0.80, 1.50, 400},
	{"ADHD Learning Lab", "Academic tutor and executive function coach helping students with ADHD develop study habits, focus, and organizational skills", 0.40, 2.50, 400},
	{"MathPro Academy", "Math tutor preparing ambitious students for advanced coursework, math olympiad, and competition-level problem solving", 0.45, 2.00, 400},
	{"CollegeReady Prep", "Test prep tutor helping high school students raise SAT and ACT scores and build strong college applications", 0.50, 2.25, 400},

	// Financial Advisory (for Piere, Origin demos)
	{"WealthPath Advisors", "Financial advisor helping individuals build retirement savings through diversified portfolio planning and long-term investment strategy", 0.80, 3.00, 600},
	{"SeedFund Capital", "Venture capital firm providing seed funding, pitch coaching, and investor introductions for early-stage startup founders", 0.40, 5.00, 600},
	{"TaxSmart CPA", "CPA firm helping small business owners minimize tax liability through strategic planning, deductions, and compliance", 0.50, 3.50, 600},
	{"EstateGuard Planning", "Estate planning attorney helping families set up trusts, wills, and wealth transfer strategies for generational inheritance", 0.45, 4.00, 600},

	// Personal Finance & Budgeting (for Piere, FlyFin demos)
	{"SaveSmart", "Personal finance app that automates savings, tracks spending, and helps people pay off credit card debt and build emergency funds", 0.80, 2.00, 400},
	{"FreelanceBooks", "Bookkeeper helping freelancers and 1099 contractors track expenses, maximize tax deductions, and file quarterly estimated taxes", 0.40, 3.50, 500},
	{"InsureRight", "Insurance marketplace helping self-employed and gig workers find affordable health, liability, and business coverage", 0.50, 3.00, 500},
	{"RetireEasy", "Retirement planning service helping self-employed individuals set up and optimize solo 401k and IRA accounts", 0.45, 2.75, 500},

	// Legal (for FreeLawChat, AskLegal demos)
	{"RightsCounsel", "Attorney providing legal advice and representation to individuals dealing with civil rights violations, employment disputes, and consumer protection issues", 0.80, 5.00, 800},
	{"FairRent Legal", "Tenant rights lawyer helping renters fight wrongful evictions, negotiate leases, and resolve disputes with landlords", 0.40, 6.00, 800},
	{"DivorceNav", "Family law attorney guiding couples through uncontested divorce, custody agreements, and co-parenting mediation", 0.50, 5.50, 800},
	{"InjuryPro Law", "Personal injury lawyer helping car accident and motorcycle crash victims file claims and negotiate insurance settlements", 0.45, 7.00, 800},

	// Developer Tools (for Phind demo)
	{"CloudDeploy", "Cloud platform providing Kubernetes orchestration, serverless deployment, and infrastructure management for engineering teams", 0.80, 4.00, 700},
	{"DevDB", "Managed database service offering PostgreSQL hosting, real-time replication, and automatic scaling for development teams", 0.40, 4.50, 700},
	{"ShipFast CI", "CI/CD platform providing fast parallel builds, automated testing, and deployment pipelines optimized for monorepo codebases", 0.50, 3.50, 700},
	{"APISentry", "Observability platform helping engineering teams detect API errors, debug latency issues, and monitor uptime in production", 0.45, 3.75, 700},

	// Dog Training
	{"GoodBoy Basics", "Dog trainer teaching basic obedience commands, leash manners, and house training to new puppy and adult dog owners", 0.80, 1.00, 300},
	{"Calm Canine Co", "Dog behaviorist helping owners of reactive, anxious, and fearful dogs through specialized desensitization and counter-conditioning programs", 0.40, 2.00, 300},
	{"AgilityPaws", "Dog trainer coaching competitive handlers and their dogs in agility courses, rally obedience, and canine sport events", 0.45, 1.75, 300},
	{"ServiceDog Academy", "Trainer certifying service dogs and therapy animals for individuals with disabilities, PTSD, and emotional support needs", 0.40, 2.50, 300},
}

func main() {
	sidecarURL := flag.String("sidecar-url", "http://localhost:8081", "URL of the embedding sidecar")
	dbPath := flag.String("db-path", "", "Path to SQLite database (empty = in-memory only)")
	seed := flag.Bool("seed", false, "Seed default advertisers on startup (only if DB is empty)")
	anthropicKey := flag.String("anthropic-key", "", "Anthropic API key for /chat proxy")
	flag.Parse()

	// Try env var for API key if flag not set
	if *anthropicKey == "" {
		*anthropicKey = os.Getenv("ANTHROPIC_API_KEY")
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
				pos, err := registry.RegisterWithBudget(s.Name, s.Intent, s.Sigma, s.BidPrice, s.Budget, "USD")
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

	router := handler.NewRouter(handler.RouterConfig{
		Registry:     registry,
		Budgets:      budgets,
		Engine:       engine,
		DB:           db,
		AnthropicKey: *anthropicKey,
	})

	log.Printf("CloudX Ad Server starting on :8080 (sidecar: %s)", *sidecarURL)
	if *anthropicKey != "" {
		log.Println("Chat proxy enabled (Anthropic API key configured)")
	}
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
