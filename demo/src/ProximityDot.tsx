export function ProximityDot({
  brightness,
  onClick,
  loading,
}: {
  brightness: number;
  onClick: () => void;
  loading?: boolean;
}) {
  if (brightness <= 0) return null;

  return (
    <button
      onClick={onClick}
      disabled={loading}
      className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs
                 transition-all cursor-pointer hover:scale-110 disabled:cursor-wait"
      style={
        {
          "--dot-brightness": brightness,
          opacity: 0.3 + brightness * 0.7,
        } as React.CSSProperties
      }
      title="Click to see a relevant ad"
    >
      <span
        className="proximity-dot inline-block w-2.5 h-2.5 rounded-full"
        style={{
          backgroundColor: `var(--color-primary)`,
          boxShadow: `0 0 ${brightness * 12}px ${brightness * 4}px var(--color-primary)`,
        }}
      />
      {loading ? (
        <span className="text-gray-500">loading...</span>
      ) : (
        <span style={{ color: "var(--color-primary)" }}>
          ad available
        </span>
      )}
    </button>
  );
}
