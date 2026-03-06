/**
 * Demo queries per publisher — multi-turn conversations that deepen naturally.
 *
 * Each conversation starts vague and narrows toward a need the user never
 * explicitly asks to solve. The dot does the work of connecting the need
 * to nearby expertise — the user doesn't have to ask for help.
 */

export interface DemoStep {
  message: string;
  /** Faked dot brightness (0 = invisible, 0.025 = barely there, 0.1 = faint, 0.45 = warm) */
  brightness: number;
}

export interface PublisherDemo {
  publisherId: string;
  tau: number;
  /** Off-topic opener (dot stays dark, demonstrates "UX stays clean") */
  offTopic: string;
  /** Multi-turn deepening conversation */
  steps: DemoStep[];
}

export const publisherDemos: PublisherDemo[] = [
  // --- Top 4 Warm Leads ---

  // Chai AI: companion chat → user reveals anxiety/sleep issues
  {
    publisherId: "chai",
    tau: 0.6,
    offTopic: "tell me about your favorite season",
    steps: [
      { message: "I've been really stressed out lately with work", brightness: 0.025 },
      { message: "I can't fall asleep most nights and it's getting worse", brightness: 0.1 },
      { message: "I feel anxious all the time and I don't know how to make it stop", brightness: 0.45 },
    ],
  },

  // Amp Code: dev tools → deployment/infra pain
  {
    publisherId: "amp",
    tau: 0.55,
    offTopic: "what's the best sci-fi movie of all time",
    steps: [
      { message: "I'm deploying a new service and the config is getting complicated", brightness: 0.025 },
      { message: "our database keeps running out of connections under load", brightness: 0.1 },
      { message: "we had two outages this week and I can't figure out where the bottleneck is", brightness: 0.45 },
    ],
  },

  // Luzia: general assistant → money/tax worries
  {
    publisherId: "luzia",
    tau: 0.6,
    offTopic: "what's a good recipe for guacamole",
    steps: [
      { message: "I've been picking up extra gig work to make ends meet", brightness: 0.025 },
      { message: "I have no idea how to keep track of all these different payments", brightness: 0.1 },
      { message: "tax season is coming and I don't even know what I owe", brightness: 0.45 },
    ],
  },

  // Kindroid: AI companion → loneliness/mental health
  {
    publisherId: "kindroid",
    tau: 0.6,
    offTopic: "if you could travel anywhere where would you go",
    steps: [
      { message: "I moved to a new city a few months ago and it's been rough", brightness: 0.025 },
      { message: "I don't really have anyone here to hang out with", brightness: 0.1 },
      { message: "some days I just sit in my apartment and feel like I'm losing it", brightness: 0.45 },
    ],
  },

  // --- 6 New Targets ---

  // Galen AI: health → cholesterol/diet concerns
  {
    publisherId: "galenai",
    tau: 0.6,
    offTopic: "what's the best way to stay hydrated",
    steps: [
      { message: "I just got my blood work back and a few things were flagged", brightness: 0.025 },
      { message: "my cholesterol is higher than it should be for my age", brightness: 0.1 },
      { message: "my doctor mentioned statins but I'd rather try changing my diet first", brightness: 0.45 },
    ],
  },

  // Autonomous: finance → inheritance/estate planning
  {
    publisherId: "autonomous",
    tau: 0.55,
    offTopic: "what's a good book to read this weekend",
    steps: [
      { message: "my grandmother passed away and left me some money", brightness: 0.025 },
      { message: "I've never dealt with this much money before and I don't know where to put it", brightness: 0.1 },
      { message: "I'm worried about the tax implications and don't want to make a mistake", brightness: 0.45 },
    ],
  },


  // Sonia: therapy → anger/relationship issues
  {
    publisherId: "sonia",
    tau: 0.6,
    offTopic: "do you have any tips for staying organized",
    steps: [
      { message: "I've been feeling really off for the past couple weeks", brightness: 0.025 },
      { message: "I keep snapping at people I care about over small things", brightness: 0.1 },
      { message: "I think I need to actually talk to someone about what's going on with me", brightness: 0.45 },
    ],
  },

  // YouLearn: education → exam prep panic
  {
    publisherId: "youlearn",
    tau: 0.6,
    offTopic: "what's the most interesting fact you know",
    steps: [
      { message: "I have a big exam coming up next week and I'm not ready", brightness: 0.025 },
      { message: "the professor's lectures don't make sense and the textbook is useless", brightness: 0.1 },
      { message: "I've watched the recordings twice and I still can't explain the core concepts", brightness: 0.45 },
    ],
  },

  // Alice: education → organic chemistry struggle
  {
    publisherId: "alice",
    tau: 0.6,
    offTopic: "what's a fun way to memorize things",
    steps: [
      { message: "I'm taking organic chemistry this semester and it's brutal", brightness: 0.025 },
      { message: "the reaction mechanisms just don't click no matter how much I study", brightness: 0.1 },
      { message: "my grade is dropping and I can't afford to fail this class", brightness: 0.45 },
    ],
  },
];
