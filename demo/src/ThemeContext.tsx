import { createContext, useContext, useEffect, useMemo } from "react";
import { getTheme, type PublisherTheme } from "./publisher-themes";

const CSS_VAR_MAP: Record<string, keyof PublisherTheme["colors"]> = {
  "--theme-primary": "primary",
  "--theme-primary-hover": "primaryHover",
  "--theme-accent": "accent",
  "--theme-bg": "background",
  "--theme-surface": "surface",
  "--theme-border": "border",
  "--theme-text": "text",
  "--theme-text-muted": "textMuted",
  "--theme-user-bubble": "userBubble",
  "--theme-user-bubble-text": "userBubbleText",
  "--theme-bot-bubble": "assistantBubble",
  "--theme-bot-bubble-text": "assistantBubbleText",
  "--theme-dot-active": "dotActive",
};

const ThemeContext = createContext<PublisherTheme>(getTheme("chai"));

export function useTheme() {
  return useContext(ThemeContext);
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = useMemo(() => {
    const params = new URLSearchParams(window.location.search);
    return getTheme(params.get("publisher") ?? "chai");
  }, []);

  useEffect(() => {
    const root = document.documentElement.style;
    for (const [cssVar, colorKey] of Object.entries(CSS_VAR_MAP)) {
      root.setProperty(cssVar, theme.colors[colorKey]);
    }
  }, [theme]);

  return (
    <ThemeContext.Provider value={theme}>{children}</ThemeContext.Provider>
  );
}
