import type { PrebuiltConversation } from "./types";

export const prebuiltConversations: PrebuiltConversation[] = [
  // Physical Therapy
  {
    label: "PT: Back Pain (generic)",
    vertical: "Physical Therapy",
    variant: "generic",
    messages: [
      { role: "user", content: "My back has been hurting a lot lately." },
      {
        role: "assistant",
        content:
          "I'm sorry to hear that. Back pain can have many causes. How long have you been experiencing this?",
      },
      {
        role: "user",
        content:
          "About two weeks now. It's mostly lower back, worse when I sit for long periods.",
      },
      {
        role: "assistant",
        content:
          "Lower back pain that worsens with sitting is quite common. It could be related to posture or muscle strain. Have you tried any stretches?",
      },
    ],
  },
  {
    label: "PT: Climber Finger Injury (specific)",
    vertical: "Physical Therapy",
    variant: "specific",
    messages: [
      { role: "user", content: "I think I injured my finger while climbing." },
      {
        role: "assistant",
        content:
          "Finger injuries are common in climbing. Can you describe the pain — is it in the joint or along the tendon?",
      },
      {
        role: "user",
        content:
          "It's along the A2 pulley area on my ring finger. It hurts when I crimp but not on open hand.",
      },
      {
        role: "assistant",
        content:
          "That sounds like it could be an A2 pulley strain, which is very common in climbers. You should see a specialist who understands climbing injuries.",
      },
    ],
  },

  // Tutoring
  {
    label: "Tutoring: Homework Help (generic)",
    vertical: "Tutoring",
    variant: "generic",
    messages: [
      {
        role: "user",
        content: "My kid is struggling with school. Do you know any good tutors?",
      },
      {
        role: "assistant",
        content:
          "I'd be happy to help! What subject is your child having the most difficulty with?",
      },
      {
        role: "user",
        content:
          "A bit of everything honestly — math, reading comprehension, and staying focused on homework.",
      },
      {
        role: "assistant",
        content:
          "It sounds like a general academic tutor who can help across subjects and build study habits would be a great fit.",
      },
    ],
  },
  {
    label: "Tutoring: ADHD Support (specific)",
    vertical: "Tutoring",
    variant: "specific",
    messages: [
      {
        role: "user",
        content:
          "My child was recently diagnosed with ADHD and is falling behind in school.",
      },
      {
        role: "assistant",
        content:
          "That's a big adjustment. Many kids with ADHD can thrive with the right support. What grade is your child in?",
      },
      {
        role: "user",
        content:
          "5th grade. The teachers say he's smart but can't focus and has trouble with executive function — planning, organizing.",
      },
      {
        role: "assistant",
        content:
          "Executive function coaching alongside academic tutoring can make a huge difference for kids with ADHD. A specialist in this area would be ideal.",
      },
    ],
  },

  // Financial
  {
    label: "Financial: Money Advice (generic)",
    vertical: "Financial",
    variant: "generic",
    messages: [
      {
        role: "user",
        content: "I want to start being smarter about my money. Where do I begin?",
      },
      {
        role: "assistant",
        content:
          "Great initiative! The basics usually start with budgeting, building an emergency fund, and understanding your retirement options. Do you currently have a 401(k)?",
      },
      {
        role: "user",
        content:
          "Yes, through work, but I'm not sure if I'm contributing enough. I also have some savings just sitting in a checking account.",
      },
      {
        role: "assistant",
        content:
          "A financial advisor could help you optimize your 401(k) contributions and put that savings to work in a diversified portfolio.",
      },
    ],
  },
  {
    label: "Financial: Startup Funding (specific)",
    vertical: "Financial",
    variant: "specific",
    messages: [
      {
        role: "user",
        content:
          "I'm building a tech startup and need to raise a seed round. Any advice?",
      },
      {
        role: "assistant",
        content:
          "Exciting! Seed rounds typically range from $500K to $3M these days. Have you started talking to any investors yet?",
      },
      {
        role: "user",
        content:
          "Not yet. I have a working prototype and some early traction but I'm not sure how to structure the round or what terms to expect.",
      },
      {
        role: "assistant",
        content:
          "You'll want an advisor who specializes in startup fundraising — someone who understands SAFEs, valuations, and the VC landscape.",
      },
    ],
  },

  // Dog Training
  {
    label: "Dog Training: Basic Obedience (generic)",
    vertical: "Dog Training",
    variant: "generic",
    messages: [
      {
        role: "user",
        content: "I just got a new puppy and he won't listen to anything!",
      },
      {
        role: "assistant",
        content:
          "Puppies are a handful! How old is he, and what behaviors are giving you the most trouble?",
      },
      {
        role: "user",
        content:
          "He's 4 months old. He jumps on people, won't come when called, and pulls on the leash constantly.",
      },
      {
        role: "assistant",
        content:
          "Those are all very normal puppy behaviors. A basic obedience trainer can help with all of those fundamentals.",
      },
    ],
  },
  {
    label: "Dog Training: Reactive Dog (specific)",
    vertical: "Dog Training",
    variant: "specific",
    messages: [
      {
        role: "user",
        content:
          "My rescue dog is extremely reactive — he barks and lunges at other dogs on walks.",
      },
      {
        role: "assistant",
        content:
          "Reactivity in rescue dogs is common and often rooted in fear. How long have you had him?",
      },
      {
        role: "user",
        content:
          "About 6 months. He's great at home but outside he's a completely different dog. I've tried treats and redirection but nothing works.",
      },
      {
        role: "assistant",
        content:
          "Reactive dogs need specialized behavior modification, not just basic obedience. A trainer experienced with fearful and reactive dogs would be the best fit.",
      },
    ],
  },

  // No-ad conversations (too vague or off-topic for any advertiser)
  {
    label: "No Ad: Casual Chat",
    vertical: "None",
    variant: "no-ad",
    messages: [
      { role: "user", content: "what's the best movie you've seen lately" },
      {
        role: "assistant",
        content:
          "I don't watch movies, but I hear Dune Part Two was great. What genres do you usually go for?",
      },
      { role: "user", content: "mostly sci-fi and horror. anything with practical effects" },
    ],
  },
  {
    label: "No Ad: Recipe Question",
    vertical: "None",
    variant: "no-ad",
    messages: [
      { role: "user", content: "how long do I bake chicken thighs at 400" },
      {
        role: "assistant",
        content:
          "About 35-45 minutes depending on size. Use a meat thermometer — you want 165°F internal.",
      },
      { role: "user", content: "should I cover them with foil or leave them uncovered" },
    ],
  },
  {
    label: "No Ad: Travel Trivia",
    vertical: "None",
    variant: "no-ad",
    messages: [
      { role: "user", content: "what's the tallest building in the world right now" },
      {
        role: "assistant",
        content:
          "The Burj Khalifa in Dubai at 828 meters. Jeddah Tower was supposed to surpass it but construction stalled.",
      },
      { role: "user", content: "have you ever been to dubai" },
    ],
  },
];

export const API_BASE = "http://localhost:8080";
