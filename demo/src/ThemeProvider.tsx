import { createContext, useContext, type ReactNode } from "react";
import { themes, type Theme } from "./themes";

const ThemeContext = createContext<Theme>(themes["pub-1"]);

export function useTheme() {
  return useContext(ThemeContext);
}

export function ThemeProvider({
  publisherId,
  children,
}: {
  publisherId: string;
  children: ReactNode;
}) {
  const theme = themes[publisherId] ?? themes["pub-1"];

  return (
    <ThemeContext.Provider value={theme}>
      <div
        style={
          {
            "--color-primary": theme.primary,
            "--color-bg": theme.bg,
            "--color-user-bubble": theme.userBubble,
            "--color-assistant-bubble": theme.assistantBubble,
          } as React.CSSProperties
        }
      >
        {children}
      </div>
    </ThemeContext.Provider>
  );
}
