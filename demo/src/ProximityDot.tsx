interface ProximityDotProps {
  brightness: number; // 0 to 1
  onClick: () => void;
  hasResult: boolean;
}

export function ProximityDot({
  brightness,
  onClick,
  hasResult,
}: ProximityDotProps) {
  if (!hasResult) return null;

  const amber = `rgba(245, 158, 11, ${0.4 + brightness * 0.6})`;
  const glowSize = 4 + brightness * 16;

  return (
    <button
      onClick={onClick}
      title="View auction details"
      style={{
        width: 28,
        height: 28,
        borderRadius: "50%",
        border: "none",
        background: amber,
        boxShadow: `0 0 ${glowSize}px ${glowSize / 2}px ${amber}`,
        cursor: "pointer",
        transition: "all 0.3s ease",
        flexShrink: 0,
      }}
    />
  );
}
