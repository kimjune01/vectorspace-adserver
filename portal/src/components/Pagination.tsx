interface PaginationProps {
  total: number;
  limit: number;
  offset: number;
  onChange: (offset: number) => void;
}

export function Pagination({ total, limit, offset, onChange }: PaginationProps) {
  const pages = Math.ceil(total / limit);
  const current = Math.floor(offset / limit);

  if (pages <= 1) return null;

  return (
    <div className="flex items-center gap-2 mt-4">
      <button
        onClick={() => onChange(Math.max(0, offset - limit))}
        disabled={offset === 0}
        className="px-3 py-1 text-sm rounded border border-slate-300 disabled:opacity-40"
      >
        Prev
      </button>
      <span className="text-sm text-slate-500">
        Page {current + 1} of {pages}
      </span>
      <button
        onClick={() => onChange(offset + limit)}
        disabled={offset + limit >= total}
        className="px-3 py-1 text-sm rounded border border-slate-300 disabled:opacity-40"
      >
        Next
      </button>
    </div>
  );
}
