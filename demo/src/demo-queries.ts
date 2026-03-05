/**
 * Demo queries per publisher — designed to showcase the tau narrative:
 *
 * Without tau: high-bidding generalists win, ads feel irrelevant.
 * With tau: only contextually relevant ads pass, UX stays clean, revenue still flows.
 */

export interface DemoQuery {
  intent: string;
  /** What the audience should notice */
  narrative: string;
}

export interface PublisherDemo {
  publisherId: string;
  tau: number;
  queries: DemoQuery[];
}

export const publisherDemos: PublisherDemo[] = [
  // --- Health (CodyMD, Doctronic, Counsel Health, August AI) ---
  {
    publisherId: "codymd",
    tau: 0.8,
    queries: [
      { intent: "my lower back has been hurting for two weeks", narrative: "PT specialists surface, not financial advisors" },
      { intent: "feeling anxious and can't sleep at night", narrative: "Mental health provider wins over high-bidding generalists" },
      { intent: "I have a rash on my arm that won't go away", narrative: "Dermatology screening beats irrelevant verticals" },
    ],
  },
  {
    publisherId: "doctronic",
    tau: 0.85,
    queries: [
      { intent: "I think I have a sinus infection", narrative: "Telehealth provider surfaces for primary care need" },
      { intent: "my child has a fever and sore throat", narrative: "Healthcare ads, not tutoring or dog training" },
    ],
  },
  {
    publisherId: "counsel-health",
    tau: 0.76,
    queries: [
      { intent: "I need to talk to someone about my depression", narrative: "Therapy provider wins — high-value, high-relevance" },
      { intent: "should I see a doctor about this knee pain", narrative: "PT and telehealth compete, irrelevant verticals excluded" },
    ],
  },
  {
    publisherId: "august-ai",
    tau: 0.85,
    queries: [
      { intent: "what foods help lower cholesterol", narrative: "Nutrition specialist surfaces for diet query" },
      { intent: "how do I know if I pulled a muscle or tore something", narrative: "PT specialists — the right ad for the moment" },
    ],
  },

  // --- Legal (FreeLawChat, AskLegal) ---
  {
    publisherId: "freelawchat",
    tau: 0.7,
    queries: [
      { intent: "my landlord is trying to evict me without notice", narrative: "Tenant rights attorney, not a financial advisor" },
      { intent: "going through a divorce and need custody advice", narrative: "Family law specialist wins the relevant auction" },
      { intent: "I was rear-ended and need to file an injury claim", narrative: "Personal injury attorney — highest value intent in the system" },
    ],
  },
  {
    publisherId: "asklegal",
    tau: 0.75,
    queries: [
      { intent: "can my employer fire me for taking medical leave", narrative: "Employment rights attorney surfaces" },
      { intent: "someone hit my parked car and drove away", narrative: "Injury law competes in the relevant pool" },
    ],
  },

  // --- Finance (Piere, FlyFin, Origin) ---
  {
    publisherId: "piere",
    tau: 0.8,
    queries: [
      { intent: "I want to start saving for retirement but don't know where to begin", narrative: "Retirement planning wins over unrelated verticals" },
      { intent: "how do I pay off credit card debt faster", narrative: "Budgeting and savings tools — native to the conversation" },
    ],
  },
  {
    publisherId: "flyfin",
    tau: 0.7,
    queries: [
      { intent: "what expenses can I deduct as a freelance designer", narrative: "Freelancer bookkeeping and tax services surface" },
      { intent: "do I need to pay quarterly estimated taxes on 1099 income", narrative: "Tax specialist beats generalist financial advisors" },
      { intent: "should I set up an LLC or stay sole proprietor", narrative: "Small business tax optimization — precise match" },
    ],
  },
  {
    publisherId: "origin",
    tau: 0.85,
    queries: [
      { intent: "how should I invest my first ten thousand dollars", narrative: "Wealth management wins in a competitive finance auction" },
      { intent: "what's the difference between a Roth IRA and traditional IRA", narrative: "Retirement planning — the right ad for someone planning their future" },
    ],
  },

  // --- Education (Brainly) ---
  {
    publisherId: "brainly",
    tau: 0.8,
    queries: [
      { intent: "my child needs a tutor for math class", narrative: "Math tutor surfaces — not a dog trainer" },
      { intent: "how do I study for the SAT effectively", narrative: "Test prep specialist wins the education auction" },
      { intent: "my child has ADHD and is falling behind in school", narrative: "ADHD learning specialist — the ad the parent actually needs" },
    ],
  },

  // --- Developer (Phind) ---
  {
    publisherId: "phind",
    tau: 0.6,
    queries: [
      { intent: "how do I set up a CI pipeline for my monorepo", narrative: "CI platform ad — feels like a recommendation, not an ad" },
      { intent: "how do I monitor my API for errors and latency", narrative: "Observability tool surfaces for a performance problem" },
      { intent: "set up kubernetes deployment pipeline", narrative: "Cloud deployment — the highest CPM vertical in dev tools" },
    ],
  },
];
