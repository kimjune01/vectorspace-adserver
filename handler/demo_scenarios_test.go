package handler

import (
	"bytes"
	"cloudx-adserver/platform"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// demoQuery defines one probe for a publisher demo.
type demoQuery struct {
	Intent        string
	Tau           float64
	ExpectWinners []string // at least one of these names should win
}

// demoScenario groups queries by publisher vertical.
type demoScenario struct {
	Publisher string
	Queries   []demoQuery
}

var demoScenarios = []demoScenario{
	{
		Publisher: "CodyMD / Health",
		Queries: []demoQuery{
			{"my lower back has been hurting for two weeks", 0.8, []string{"AllMotion PT", "Peak PT", "QuickClinic"}},
			{"feeling anxious and can't sleep at night", 0.8, []string{"MindBridge Therapy"}},
			{"I have a rash on my arm that won't go away", 0.8, []string{"DermCheck AI"}},
		},
	},
	{
		Publisher: "Doctronic / Health",
		Queries: []demoQuery{
			{"I think I have a sinus infection", 0.85, []string{"DermCheck AI", "QuickClinic"}},
		},
	},
	{
		Publisher: "Counsel Health",
		Queries: []demoQuery{
			{"I need to talk to someone about my depression", 0.76, []string{"MindBridge Therapy"}},
			{"should I see a doctor about this knee pain", 0.8, []string{"AllMotion PT", "QuickClinic"}},
		},
	},
	{
		Publisher: "August AI / Health",
		Queries: []demoQuery{
			{"what foods help lower cholesterol", 0.85, []string{"NutriPlan Pro"}},
			{"how do I know if I pulled a muscle or tore something", 0.9, []string{"AllMotion PT", "Peak PT", "QuickClinic"}},
		},
	},
	{
		Publisher: "FreeLawChat / Legal",
		Queries: []demoQuery{
			{"my landlord is trying to evict me without notice", 0.7, []string{"FairRent Legal"}},
			{"going through a divorce and need custody advice", 0.7, []string{"DivorceNav"}},
			{"I was rear-ended and need to file an injury claim", 0.95, []string{"RightsCounsel", "InjuryPro Law"}},
		},
	},
	{
		Publisher: "FlyFin / Freelancer Finance",
		Queries: []demoQuery{
			{"what expenses can I deduct as a freelance designer", 0.7, []string{"FreelanceBooks"}},
			{"do I need to pay quarterly estimated taxes on 1099 income", 0.7, []string{"FreelanceBooks", "TaxSmart CPA"}},
			{"should I set up an LLC or stay sole proprietor", 0.7, []string{"TaxSmart CPA", "FreelanceBooks"}},
		},
	},
	{
		Publisher: "Piere / Personal Finance",
		Queries: []demoQuery{
			{"I want to start saving for retirement", 0.8, []string{"WealthPath Advisors", "SaveSmart", "RetireEasy"}},
			{"how do I pay off credit card debt faster", 0.8, []string{"SaveSmart"}},
		},
	},
	{
		Publisher: "Origin / Finance",
		Queries: []demoQuery{
			{"how should I invest my first ten thousand dollars", 0.85, []string{"WealthPath Advisors"}},
			{"what's the difference between a Roth IRA and traditional IRA", 0.85, []string{"WealthPath Advisors", "RetireEasy"}},
		},
	},
	{
		Publisher: "Brainly / Education",
		Queries: []demoQuery{
			{"my child needs a tutor for math class", 0.8, []string{"BrightMinds Tutoring", "MathPro Academy", "CollegeReady Prep"}},
			{"how do I study for the SAT effectively", 0.8, []string{"BrightMinds Tutoring", "CollegeReady Prep"}},
			{"my child has ADHD and is falling behind in school", 0.8, []string{"BrightMinds Tutoring", "ADHD Learning Lab"}},
		},
	},
	{
		Publisher: "Phind / Developer",
		Queries: []demoQuery{
			{"how do I set up a CI pipeline for my monorepo", 0.6, []string{"ShipFast CI", "CloudDeploy"}},
			{"how do I monitor my API for errors and latency", 0.9, []string{"CloudDeploy", "APISentry", "ShipFast CI"}},
			{"set up kubernetes deployment pipeline", 0.9, []string{"CloudDeploy", "ShipFast CI"}},
		},
	},
}

// seedAdvertisers mirrors cmd/server/main.go's full roster.
var demoSeedAdvertisers = []struct {
	Name     string
	Intent   string
	Sigma    float64
	BidPrice float64
	Budget   float64
}{
	// PT
	{"Peak PT", "Physical therapist helping runners and endurance athletes recover from sports injuries through targeted rehab programs", 0.50, 2.50, 500},
	{"AllMotion PT", "Physical therapist treating back pain, posture problems, and general musculoskeletal issues through movement therapy and hands-on rehab", 0.80, 2.00, 500},
	{"ClimbStrong Rehab", "Physical therapist specializing in finger, hand, and upper extremity injuries common among rock climbers and bouldering athletes", 0.40, 3.00, 500},
	{"NeuroMove PT", "Physical therapist helping stroke survivors and traumatic brain injury patients regain movement through neurological rehabilitation", 0.45, 2.75, 500},
	// Health
	{"QuickClinic", "Online doctor providing telehealth visits, prescriptions, and urgent care consultations for patients who need medical attention fast", 0.80, 3.50, 600},
	{"MindBridge Therapy", "Licensed therapist offering CBT and counseling sessions to adults struggling with anxiety, depression, and insomnia", 0.50, 4.00, 600},
	{"DermCheck AI", "Dermatology screening tool that helps people identify skin rashes, suspicious moles, and other skin conditions from photos", 0.40, 3.75, 600},
	{"NutriPlan Pro", "Registered dietitian creating personalized meal plans and nutrition guidance for people managing cholesterol, diabetes, or weight loss", 0.45, 2.25, 500},
	// Tutoring
	{"BrightMinds Tutoring", "Academic tutor helping K-12 students improve grades in math, science, reading, and other school subjects through one-on-one lessons", 0.80, 1.50, 400},
	{"ADHD Learning Lab", "Academic tutor and executive function coach helping students with ADHD develop study habits, focus, and organizational skills", 0.40, 2.50, 400},
	{"MathPro Academy", "Math tutor preparing ambitious students for advanced coursework, math olympiad, and competition-level problem solving", 0.45, 2.00, 400},
	{"CollegeReady Prep", "Test prep tutor helping high school students raise SAT and ACT scores and build strong college applications", 0.50, 2.25, 400},
	// Financial Advisory
	{"WealthPath Advisors", "Financial advisor helping individuals build retirement savings through diversified portfolio planning and long-term investment strategy", 0.80, 3.00, 600},
	{"SeedFund Capital", "Venture capital firm providing seed funding, pitch coaching, and investor introductions for early-stage startup founders", 0.40, 5.00, 600},
	{"TaxSmart CPA", "CPA firm helping small business owners minimize tax liability through strategic planning, deductions, and compliance", 0.50, 3.50, 600},
	{"EstateGuard Planning", "Estate planning attorney helping families set up trusts, wills, and wealth transfer strategies for generational inheritance", 0.45, 4.00, 600},
	// Personal Finance
	{"SaveSmart", "Personal finance app that automates savings, tracks spending, and helps people pay off credit card debt and build emergency funds", 0.80, 2.00, 400},
	{"FreelanceBooks", "Bookkeeper helping freelancers and 1099 contractors track expenses, maximize tax deductions, and file quarterly estimated taxes", 0.40, 3.50, 500},
	{"InsureRight", "Insurance marketplace helping self-employed and gig workers find affordable health, liability, and business coverage", 0.50, 3.00, 500},
	{"RetireEasy", "Retirement planning service helping self-employed individuals set up and optimize solo 401k and IRA accounts", 0.45, 2.75, 500},
	// Legal
	{"RightsCounsel", "Attorney providing legal advice and representation to individuals dealing with civil rights violations, employment disputes, and consumer protection issues", 0.80, 5.00, 800},
	{"FairRent Legal", "Tenant rights lawyer helping renters fight wrongful evictions, negotiate leases, and resolve disputes with landlords", 0.40, 6.00, 800},
	{"DivorceNav", "Family law attorney guiding couples through uncontested divorce, custody agreements, and co-parenting mediation", 0.50, 5.50, 800},
	{"InjuryPro Law", "Personal injury lawyer helping car accident and motorcycle crash victims file claims and negotiate insurance settlements", 0.45, 7.00, 800},
	// Developer
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

// requireSidecar checks if the real embedding sidecar is reachable.
func requireSidecar(t *testing.T) string {
	t.Helper()
	resp, err := http.Get("http://localhost:8081/health")
	if err != nil {
		t.Skip("embedding sidecar not running on :8081 — skipping integration test")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skip("embedding sidecar unhealthy — skipping integration test")
	}
	return "http://localhost:8081"
}

// setupDemoRouter creates a full router with all 32 seed advertisers using the real sidecar.
func setupDemoRouter(t *testing.T, sidecarURL string) http.Handler {
	t.Helper()

	db, err := platform.NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	embedder := platform.NewEmbedder(sidecarURL)
	registry := platform.NewPositionRegistry(embedder)
	if err := registry.SetDB(db); err != nil {
		t.Fatal(err)
	}
	budgets := platform.NewBudgetTracker()
	if err := budgets.SetDB(db); err != nil {
		t.Fatal(err)
	}

	for _, s := range demoSeedAdvertisers {
		pos, err := registry.RegisterWithBudget(s.Name, s.Intent, s.Sigma, s.BidPrice, s.Budget, "USD", "")
		if err != nil {
			t.Fatalf("seed %s: %v", s.Name, err)
		}
		budgets.Set(pos.ID, s.Budget, "USD")
	}

	engine := platform.NewAuctionEngine(registry, budgets, embedder)
	engine.DB = db

	return NewRouter(RouterConfig{
		Registry: registry,
		Budgets:  budgets,
		Engine:   engine,
		DB:       db,
	})
}

func TestDemoScenarios(t *testing.T) {
	sidecarURL := requireSidecar(t)
	router := setupDemoRouter(t, sidecarURL)

	for _, scenario := range demoScenarios {
		t.Run(scenario.Publisher, func(t *testing.T) {
			for _, q := range scenario.Queries {
				t.Run(q.Intent, func(t *testing.T) {
					// Run with tau
					body, _ := json.Marshal(map[string]interface{}{"intent": q.Intent, "tau": q.Tau})
					req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					if w.Code != http.StatusOK {
						t.Fatalf("status %d: %s", w.Code, w.Body.String())
					}

					var resp struct {
						Winner    *struct{ Name string } `json:"winner"`
						BidCount  int                    `json:"bid_count"`
						AllBidders []struct {
							Name       string  `json:"name"`
							DistanceSq float64 `json:"distance_sq"`
						} `json:"all_bidders"`
					}
					if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
						t.Fatal(err)
					}

					if resp.Winner == nil {
						t.Fatal("no winner")
					}

					// Tau should filter — fewer than 32 bidders
					if resp.BidCount >= 32 {
						t.Errorf("tau=%.2f did not filter: bid_count=%d (expected < 32)", q.Tau, resp.BidCount)
					}

					// Winner should be one of the expected names
					found := false
					for _, name := range q.ExpectWinners {
						if resp.Winner.Name == name {
							found = true
							break
						}
					}
					if !found {
						// Log all bidders for debugging
						var bidderNames []string
						for _, b := range resp.AllBidders {
							bidderNames = append(bidderNames, fmt.Sprintf("%s(d²=%.3f)", b.Name, b.DistanceSq))
						}
						t.Errorf("winner %q not in expected %v\n  tau=%.2f, bid_count=%d, all bidders: %v",
							resp.Winner.Name, q.ExpectWinners, q.Tau, resp.BidCount, bidderNames)
					}
				})
			}
		})
	}
}

func TestDemoTauFiltersIrrelevant(t *testing.T) {
	sidecarURL := requireSidecar(t)
	router := setupDemoRouter(t, sidecarURL)

	// Without tau, the highest-bidding generalist wins regardless of relevance.
	// With tau, only the relevant vertical survives.
	cases := []struct {
		Name       string
		Intent     string
		Tau        float64
		BadWinners []string // these should NOT win with tau
	}{
		{
			"health query excludes legal",
			"my lower back has been hurting for two weeks",
			0.8,
			[]string{"RightsCounsel", "FairRent Legal", "DivorceNav", "InjuryPro Law", "CloudDeploy", "DevDB", "ShipFast CI", "APISentry"},
		},
		{
			"legal query excludes health",
			"my landlord is trying to evict me without notice",
			0.7,
			[]string{"Peak PT", "AllMotion PT", "MathPro Academy", "GoodBoy Basics", "CloudDeploy"},
		},
		{
			"dev query excludes finance",
			"how do I set up a CI pipeline for my monorepo",
			0.6,
			[]string{"WealthPath Advisors", "SeedFund Capital", "RightsCounsel", "GoodBoy Basics"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"intent": tc.Intent, "tau": tc.Tau})
			req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}

			var resp struct {
				Winner     *struct{ Name string } `json:"winner"`
				AllBidders []struct{ Name string } `json:"all_bidders"`
			}
			json.NewDecoder(w.Body).Decode(&resp)

			// None of the bad winners should appear in the filtered results
			bidderSet := make(map[string]bool)
			for _, b := range resp.AllBidders {
				bidderSet[b.Name] = true
			}
			for _, bad := range tc.BadWinners {
				if bidderSet[bad] {
					t.Errorf("%q should have been filtered out by tau=%.2f but is in results", bad, tc.Tau)
				}
			}
		})
	}
}

func TestDemoWithoutTauGeneralistWins(t *testing.T) {
	sidecarURL := requireSidecar(t)
	router := setupDemoRouter(t, sidecarURL)

	// Without tau, the highest-bidding wide-sigma advertiser tends to win
	// regardless of query relevance — this is what tau is designed to prevent.
	queries := []string{
		"my lower back hurts",
		"I need a divorce lawyer",
		"help me study for the SAT",
	}

	for _, intent := range queries {
		t.Run(intent, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"intent": intent})
			req := httptest.NewRequest("POST", "/ad-request", bytes.NewReader(body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				respBody, _ := io.ReadAll(w.Body)
				t.Fatalf("status %d: %s", w.Code, string(respBody))
			}

			var resp struct {
				BidCount int `json:"bid_count"`
			}
			json.NewDecoder(w.Body).Decode(&resp)

			// All 32 bidders should participate without tau
			if resp.BidCount != 32 {
				t.Errorf("without tau: bid_count=%d, want 32", resp.BidCount)
			}
		})
	}
}
