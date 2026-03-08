import { conversations } from "./conversations";

export function ConversationPicker({
  onSelect,
  onReset,
}: {
  onSelect: (index: number) => void;
  onReset: () => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <select
        onChange={(e) => {
          const v = e.target.value;
          if (v !== "") onSelect(Number(v));
        }}
        defaultValue=""
        className="text-sm border border-gray-300 rounded-md px-2 py-1 bg-white"
      >
        <option value="" disabled>
          Load a conversation...
        </option>
        {conversations.map((c, i) => (
          <option key={c.label} value={i}>
            {c.label}
            {c.offTopic ? " (off-topic)" : ""}
          </option>
        ))}
      </select>
      <button
        onClick={onReset}
        className="text-sm text-gray-500 hover:text-gray-700 underline"
      >
        Reset
      </button>
    </div>
  );
}
