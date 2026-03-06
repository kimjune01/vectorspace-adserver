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
  // --- Top 4 Warm Leads ---

  // Chai AI: dark bg, teal (#1AC5B3) primary
  {
    id: "chai",
    name: "Chai",
    vertical: "Social AI",
    logo: "Chai",
    colors: {
      primary: "#1AC5B3",
      primaryHover: "#15a89a",
      accent: "#1AC5B3",
      background: "#101010",
      surface: "#1a1a1a",
      border: "#2a2a2a",
      text: "#f2f2f2",
      textMuted: "#9a9a9a",
      userBubble: "#1AC5B3",
      userBubbleText: "#000000",
      assistantBubble: "#1a1a1a",
      assistantBubbleText: "#f2f2f2",
      dotActive: "rgba(26, 197, 179, VAR)",
    },
    defaultTau: 0.8,
    greeting: "Hey! Who would you like to talk to today?",
  },

  // Amp Code: dark bg, orange (#F6833B) primary
  {
    id: "amp",
    name: "Amp",
    vertical: "Developer Tools",
    logo: "Amp",
    colors: {
      primary: "#F6833B",
      primaryHover: "#e06f28",
      accent: "#FABD2F",
      background: "#171717",
      surface: "#1e1e1e",
      border: "#2e2e2e",
      text: "#f2f2f2",
      textMuted: "#9a9a9a",
      userBubble: "#F6833B",
      userBubbleText: "#000000",
      assistantBubble: "#1e1e1e",
      assistantBubbleText: "#f2f2f2",
      dotActive: "rgba(246, 131, 59, VAR)",
    },
    defaultTau: 0.7,
    greeting: "What are you working on?",
  },

  // Luzia: light bg, blue (#3D46FB) primary, coral accent
  {
    id: "luzia",
    name: "Luzia",
    vertical: "General Assistant",
    logo: "Luzia",
    colors: {
      primary: "#3D46FB",
      primaryHover: "#2e36d9",
      accent: "#D97F64",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e5e7eb",
      text: "#0f172a",
      textMuted: "#6b7280",
      userBubble: "#3D46FB",
      userBubbleText: "#ffffff",
      assistantBubble: "#f3f4f6",
      assistantBubbleText: "#0f172a",
      dotActive: "rgba(61, 70, 251, VAR)",
    },
    defaultTau: 0.8,
    greeting: "Hi! I'm Luzia, your AI assistant. How can I help?",
  },

  // Kindroid: dark bg, purple-to-coral gradient, purple (#8B6DFF) primary
  {
    id: "kindroid",
    name: "Kindroid",
    vertical: "AI Companion",
    logo: "Kindroid",
    colors: {
      primary: "#8B6DFF",
      primaryHover: "#7558e6",
      accent: "#FE8484",
      background: "#0a0a0a",
      surface: "#161616",
      border: "#2a2a2a",
      text: "#f2f2f2",
      textMuted: "#9a9a9a",
      userBubble: "#8B6DFF",
      userBubbleText: "#ffffff",
      assistantBubble: "#161616",
      assistantBubbleText: "#f2f2f2",
      dotActive: "rgba(139, 109, 255, VAR)",
    },
    defaultTau: 0.75,
    greeting: "Hey, I'm here. What's on your mind?",
  },

  // --- 6 New Targets ---

  // Galen AI: light bg, blue (#4D65FF) primary, pink accent
  {
    id: "galenai",
    name: "Galen AI",
    vertical: "Health",
    logo: "Galen",
    colors: {
      primary: "#4D65FF",
      primaryHover: "#3a50e6",
      accent: "#FF99BB",
      background: "#F0F1F2",
      surface: "#ffffff",
      border: "#e2e5e9",
      text: "#0f172a",
      textMuted: "#6b7280",
      userBubble: "#4D65FF",
      userBubbleText: "#ffffff",
      assistantBubble: "#ffffff",
      assistantBubbleText: "#0f172a",
      dotActive: "rgba(77, 101, 255, VAR)",
    },
    defaultTau: 0.8,
    greeting: "Hi, I'm Galen. How are you feeling today?",
  },

  // Autonomous: light minimal, dark text, clean finance
  {
    id: "autonomous",
    name: "Autonomous",
    vertical: "Finance",
    logo: "Autonomous",
    colors: {
      primary: "#1a1a1a",
      primaryHover: "#333333",
      accent: "#1a1a1a",
      background: "#F2F2F2",
      surface: "#ffffff",
      border: "#e0e0e0",
      text: "#1a1a1a",
      textMuted: "#6b7280",
      userBubble: "#1a1a1a",
      userBubbleText: "#ffffff",
      assistantBubble: "#ffffff",
      assistantBubbleText: "#1a1a1a",
      dotActive: "rgba(26, 26, 26, VAR)",
    },
    defaultTau: 0.7,
    greeting: "Welcome. Let's take control of your finances.",
  },


  // Sonia: cream bg (#F8F6F4), deep teal (#0B2B24) primary
  {
    id: "sonia",
    name: "Sonia",
    vertical: "Therapy",
    logo: "Sonia",
    colors: {
      primary: "#0B2B24",
      primaryHover: "#071e19",
      accent: "#ADD4FF",
      background: "#F8F6F4",
      surface: "#ffffff",
      border: "#e2dfd6",
      text: "#0B2B24",
      textMuted: "#6b7280",
      userBubble: "#0B2B24",
      userBubbleText: "#ffffff",
      assistantBubble: "#eeece5",
      assistantBubbleText: "#0B2B24",
      dotActive: "rgba(11, 43, 36, VAR)",
    },
    defaultTau: 0.75,
    greeting: "Hi, I'm Sonia. What would you like to talk about today?",
  },

  // YouLearn: light bg, cyan (#19FFDE) accent, dark text
  {
    id: "youlearn",
    name: "YouLearn",
    vertical: "Education",
    logo: "YouLearn",
    colors: {
      primary: "#0099FF",
      primaryHover: "#0080d6",
      accent: "#19FFDE",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e5e7eb",
      text: "#0f172a",
      textMuted: "#6b7280",
      userBubble: "#0099FF",
      userBubbleText: "#ffffff",
      assistantBubble: "#f3f4f6",
      assistantBubbleText: "#0f172a",
      dotActive: "rgba(0, 153, 255, VAR)",
    },
    defaultTau: 0.8,
    greeting: "What are you studying today?",
  },

  // Alice: light bg, orange (#FA5705) primary, playful
  {
    id: "alice",
    name: "Alice",
    vertical: "Education",
    logo: "Alice",
    colors: {
      primary: "#FA5705",
      primaryHover: "#e04d04",
      accent: "#328DE2",
      background: "#ffffff",
      surface: "#ffffff",
      border: "#e5e7eb",
      text: "#0f172a",
      textMuted: "#6b7280",
      userBubble: "#FA5705",
      userBubbleText: "#ffffff",
      assistantBubble: "#f3f4f6",
      assistantBubbleText: "#0f172a",
      dotActive: "rgba(250, 87, 5, VAR)",
    },
    defaultTau: 0.8,
    greeting: "Ready to study? Upload your materials or ask me anything!",
  },
];

export function getTheme(id: string): PublisherTheme {
  const lower = id.toLowerCase();
  return publisherThemes.find((t) => t.id === lower) ?? publisherThemes[0];
}
