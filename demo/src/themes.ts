export interface Theme {
  name: string;
  publisherId: string;
  greeting: string;
  primary: string;
  bg: string;
  userBubble: string;
  assistantBubble: string;
}

export const themes: Record<string, Theme> = {
  "pub-1": {
    name: "HealthChat AI",
    publisherId: "pub-1",
    greeting:
      "Hi! I'm HealthChat AI. I can help you explore health topics and find useful resources. What's on your mind?",
    primary: "#2563eb",
    bg: "#f0f7ff",
    userBubble: "#2563eb",
    assistantBubble: "#e5e7eb",
  },
  "pub-2": {
    name: "MindfulBot",
    publisherId: "pub-2",
    greeting:
      "Welcome to MindfulBot. I'm here to help you navigate wellness and mental health topics. How are you feeling today?",
    primary: "#7c3aed",
    bg: "#f5f3ff",
    userBubble: "#7c3aed",
    assistantBubble: "#ede9fe",
  },
};
