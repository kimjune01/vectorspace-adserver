import { useTheme } from "./ThemeContext";

interface ProximityDotProps {
  brightness: number; // 0 to 1
  onClick: () => void;
  hasResult: boolean;
  expanded?: boolean;
}

export function ProximityDot({
  brightness,
  onClick,
  hasResult,
  expanded,
}: ProximityDotProps) {
  const theme = useTheme();
  if (!hasResult && brightness <= 0) return null;

  const alpha = 0.4 + brightness * 0.6;
  const dotColor = theme.colors.dotActive.replace("VAR", String(alpha));
  const glowColor = theme.colors.dotActive.replace("VAR", String(alpha * 0.5));
  const glowSize = 4 + brightness * 16;

  return (
    <button
      onClick={onClick}
      title="View auction details"
      style={{
        width: 6,
        height: 6,
        borderRadius: "50%",
        border: "none",
        background: dotColor,
        boxShadow: `0 0 ${glowSize}px ${glowSize / 2}px ${glowColor}`,
        cursor: "pointer",
        flexShrink: 0,
        animation: expanded
          ? "proximity-pulse-expanded 2s ease-in-out infinite"
          : "proximity-pulse 2s ease-in-out infinite",
        transition: "transform 0.3s ease",
      }}
    />
  );
}
