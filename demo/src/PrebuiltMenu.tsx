import type { PrebuiltConversation } from "./types";
import { prebuiltConversations } from "./data";

interface PrebuiltMenuProps {
  onSelect: (conv: PrebuiltConversation) => void;
  onReset: () => void;
}

export function PrebuiltMenu({ onSelect, onReset }: PrebuiltMenuProps) {
  return (
    <div className="flex gap-2 items-center">
      <select
        className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] bg-white cursor-pointer"
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
      <button
        className="px-3 py-1.5 rounded-md border border-slate-300 bg-white text-[13px] cursor-pointer"
        onClick={onReset}
      >
        Reset
      </button>
    </div>
  );
}
