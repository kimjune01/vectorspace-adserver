export interface PublisherTheme {
  id: string;
  name: string;
  vertical: string;
  logo: string; // text fallback until real logos
  colors: {
    primary: string;
    primaryHover: string;
    accent: string;
    background: string;
    surface: string;
    border: string;
    text: string;
    textMuted: string;
    userBubble: string;
    userBubbleText: string;
    assistantBubble: string;
    assistantBubbleText: string;
    dotActive: string;
  };
  defaultTau: number;
  greeting: string;
}

export const publisherThemes: PublisherTheme[] = [
  // --- Health ---
  // CodyMD: minimal white bg, deep navy links (#141e6d), warm stone muted (#787168)
  {
    id: "codymd",
    name: "CodyMD",
    vertical: "Health",
    logo: "CodyMD",
    colors: {
      primary: "#141e6d",
      primaryHover: "#0f1752",
      accent: "#4f5bd5",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e2e8f0",
      text: "#0f172a",
      textMuted: "#787168",
      userBubble: "#141e6d",
      userBubbleText: "#ffffff",
      assistantBubble: "#f5f5f4",
      assistantBubbleText: "#0f172a",
      dotActive: "rgba(20, 30, 109, VAR)",
    },
    defaultTau: 0.8,
    greeting: "How can I help you today?",
  },
  // Doctronic: blue #0960e1 on white, clean medical
  {
    id: "doctronic",
    name: "Doctronic",
    vertical: "Health",
    logo: "Doctronic",
    colors: {
      primary: "#0960e1",
      primaryHover: "#074bb3",
      accent: "#3b82f6",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e5e7eb",
      text: "#0c0e12",
      textMuted: "#6b7280",
      userBubble: "#0960e1",
      userBubbleText: "#ffffff",
      assistantBubble: "#f3f4f6",
      assistantBubbleText: "#0c0e12",
      dotActive: "rgba(9, 96, 225, VAR)",
    },
    defaultTau: 0.7,
    greeting: "I'm your private and personal AI doctor. How can I help you today?",
  },
  // Counsel Health: navy #243866 on warm cream #f6f4ed
  {
    id: "counsel-health",
    name: "Counsel Health",
    vertical: "Health",
    logo: "Counsel",
    colors: {
      primary: "#243866",
      primaryHover: "#1b2a4d",
      accent: "#2f80ff",
      background: "#f6f4ed",
      surface: "#ffffff",
      border: "#e2dfd6",
      text: "#1c1304",
      textMuted: "#6b7280",
      userBubble: "#243866",
      userBubbleText: "#ffffff",
      assistantBubble: "#eeece5",
      assistantBubbleText: "#1c1304",
      dotActive: "rgba(36, 56, 102, VAR)",
    },
    defaultTau: 0.75,
    greeting: "Chat for free with our medical AI. What can I help with?",
  },
  // August AI: teal #08907c, lime accent #cffb20, white bg
  {
    id: "august-ai",
    name: "August AI",
    vertical: "Health",
    logo: "august",
    colors: {
      primary: "#08907c",
      primaryHover: "#067564",
      accent: "#cffb20",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e5e7eb",
      text: "#111111",
      textMuted: "#6b7280",
      userBubble: "#08907c",
      userBubbleText: "#ffffff",
      assistantBubble: "#f4f5f5",
      assistantBubbleText: "#111111",
      dotActive: "rgba(8, 144, 124, VAR)",
    },
    defaultTau: 0.85,
    greeting: "Ask anything for free. I'm your 24/7 health companion.",
  },

  // --- Legal ---
  // FreeLawChat: purple #7b29d9, cyan accents, light cyan bg #f2fdff
  {
    id: "freelawchat",
    name: "FreeLawChat",
    vertical: "Legal",
    logo: "FreeLawChat",
    colors: {
      primary: "#7b29d9",
      primaryHover: "#6321b0",
      accent: "#00d4ff",
      background: "#f2fdff",
      surface: "#ffffff",
      border: "#c4f0ff",
      text: "#000000",
      textMuted: "#6b7280",
      userBubble: "#7b29d9",
      userBubbleText: "#ffffff",
      assistantBubble: "#f0f0ff",
      assistantBubbleText: "#000000",
      dotActive: "rgba(123, 41, 217, VAR)",
    },
    defaultTau: 0.6,
    greeting: "Hi there! I'm here to help with any legal assistance you may have.",
  },
  // AskLegal.bot: site was down, keeping reasonable legal-themed defaults
  {
    id: "asklegal",
    name: "AskLegal.bot",
    vertical: "Legal",
    logo: "AskLegal",
    colors: {
      primary: "#1e40af",
      primaryHover: "#1e3a8a",
      accent: "#3b82f6",
      background: "#f8fafc",
      surface: "#ffffff",
      border: "#cbd5e1",
      text: "#0f172a",
      textMuted: "#64748b",
      userBubble: "#1e40af",
      userBubbleText: "#ffffff",
      assistantBubble: "#f1f5f9",
      assistantBubbleText: "#0f172a",
      dotActive: "rgba(30, 64, 175, VAR)",
    },
    defaultTau: 0.65,
    greeting: "Ask me any legal question. I'll help you understand your rights.",
  },

  // --- Finance ---
  // Piere: deep purple #321ebe, dark bg #121317, green accent #87d950
  {
    id: "piere",
    name: "Piere",
    vertical: "Finance",
    logo: "Piere",
    colors: {
      primary: "#321ebe",
      primaryHover: "#2a18a0",
      accent: "#87d950",
      background: "#121317",
      surface: "#1e1f25",
      border: "#2e2f38",
      text: "#f2f2f2",
      textMuted: "rgba(255, 255, 255, 0.5)",
      userBubble: "#321ebe",
      userBubbleText: "#ffffff",
      assistantBubble: "#1e1f25",
      assistantBubbleText: "#f2f2f2",
      dotActive: "rgba(135, 217, 80, VAR)",
    },
    defaultTau: 0.7,
    greeting: "Hey! Let's make your money work smarter. What's on your mind?",
  },
  // FlyFin: cyan #4affe0 on dark #121212, warm gold banner
  {
    id: "flyfin",
    name: "FlyFin",
    vertical: "Finance",
    logo: "FLYFIN",
    colors: {
      primary: "#4affe0",
      primaryHover: "#33e6c8",
      accent: "#4affe0",
      background: "#121212",
      surface: "#1a1a1a",
      border: "#2a2a2a",
      text: "#eaeaea",
      textMuted: "#9a9a9a",
      userBubble: "#4affe0",
      userBubbleText: "#0d0f0f",
      assistantBubble: "#1a1a1a",
      assistantBubbleText: "#eaeaea",
      dotActive: "rgba(74, 255, 224, VAR)",
    },
    defaultTau: 0.65,
    greeting: "Welcome to FlyFin. Ask me anything about freelancer taxes.",
  },
  // Origin Financial: white on dark #0f1011, premium minimal
  {
    id: "origin",
    name: "Origin Financial",
    vertical: "Finance",
    logo: "Origin",
    colors: {
      primary: "#ffffff",
      primaryHover: "#e0e0e0",
      accent: "#ffffff",
      background: "#0f1011",
      surface: "#1a1b1c",
      border: "#2a2a2a",
      text: "#fafafa",
      textMuted: "#5b5b5b",
      userBubble: "#ffffff",
      userBubbleText: "#000000",
      assistantBubble: "#1a1b1c",
      assistantBubbleText: "#fafafa",
      dotActive: "rgba(255, 255, 255, VAR)",
    },
    defaultTau: 0.7,
    greeting: "Hi, I'm your personal AI Financial Advisor. How can I help?",
  },

  // --- Education ---
  // Brainly: black #4c4c4c buttons, green #6cc644 CTA, white bg
  {
    id: "brainly",
    name: "Brainly",
    vertical: "Education",
    logo: "BRAINLY",
    colors: {
      primary: "#4c4c4c",
      primaryHover: "#333333",
      accent: "#6cc644",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e5e7eb",
      text: "#000000",
      textMuted: "#46535f",
      userBubble: "#4c4c4c",
      userBubbleText: "#ffffff",
      assistantBubble: "#f3f4f6",
      assistantBubbleText: "#000000",
      dotActive: "rgba(108, 198, 68, VAR)",
    },
    defaultTau: 0.8,
    greeting: "What subject do you need help with?",
  },
  // Phind: black primary, minimal dark dev tool (Cloudflare blocked, using known brand)
  {
    id: "phind",
    name: "Phind",
    vertical: "Developer",
    logo: "Phind",
    colors: {
      primary: "#000000",
      primaryHover: "#1a1a1a",
      accent: "#a1a1aa",
      background: "#0a0a0a",
      surface: "#141414",
      border: "#262626",
      text: "#fafafa",
      textMuted: "#a1a1aa",
      userBubble: "#262626",
      userBubbleText: "#fafafa",
      assistantBubble: "#141414",
      assistantBubbleText: "#fafafa",
      dotActive: "rgba(161, 161, 170, VAR)",
    },
    defaultTau: 0.5,
    greeting: "What are you building? Ask me anything.",
  },
];

export function getTheme(id: string): PublisherTheme {
  return publisherThemes.find((t) => t.id === id) ?? publisherThemes[0];
}
