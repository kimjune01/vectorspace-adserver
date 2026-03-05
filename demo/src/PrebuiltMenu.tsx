import type { PrebuiltConversation } from "./types";
import { prebuiltConversations } from "./data";

interface PrebuiltMenuProps {
  onSelect: (conv: PrebuiltConversation) => void;
  onReset: () => void;
}

export function PrebuiltMenu({ onSelect, onReset }: PrebuiltMenuProps) {
  return (
    <div style={styles.container}>
      <select
        style={styles.select}
        onChange={(e) => {
          const idx = parseInt(e.target.value);
          if (idx >= 0) onSelect(prebuiltConversations[idx]);
          e.target.value = "-1";
        }}
        defaultValue="-1"
      >
        <option value="-1" disabled>
          Load prebuilt conversation...
        </option>
        {prebuiltConversations.map((conv, i) => (
          <option key={i} value={i}>
            {conv.label}
          </option>
        ))}
      </select>
      <button style={styles.resetBtn} onClick={onReset}>
        Reset
      </button>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: "flex",
    gap: "8px",
    alignItems: "center",
  },
  select: {
    padding: "6px 10px",
    borderRadius: "6px",
    border: "1px solid #cbd5e1",
    fontSize: "13px",
    background: "white",
    cursor: "pointer",
  },
  resetBtn: {
    padding: "6px 12px",
    borderRadius: "6px",
    border: "1px solid #cbd5e1",
    background: "white",
    fontSize: "13px",
    cursor: "pointer",
  },
};
