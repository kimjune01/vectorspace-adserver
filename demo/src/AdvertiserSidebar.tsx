import { useState, useEffect } from "react";
import type { Advertiser } from "./types";

interface AdvertiserSidebarProps {
  advertisers: Advertiser[];
  onUpdate: (
    id: string,
    updates: Partial<{
      name: string;
      intent: string;
      sigma: number;
      bid_price: number;
    }>
  ) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
  onAdd: (adv: {
    name: string;
    intent: string;
    sigma: number;
    bid_price: number;
    budget: number;
  }) => Promise<void>;
}

export function AdvertiserSidebar({
  advertisers,
  onUpdate,
  onDelete,
  onAdd,
}: AdvertiserSidebarProps) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="flex justify-between items-center px-4 py-3 border-b border-slate-200">
        <h3 className="m-0 text-base">Advertiser Roster</h3>
        <button
          className="px-3 py-1 rounded-md border border-slate-300 bg-white text-[13px] cursor-pointer"
          onClick={() => setShowAdd(!showAdd)}
        >
          {showAdd ? "Cancel" : "+ Add"}
        </button>
      </div>

      {showAdd && (
        <AddForm
          onAdd={async (adv) => {
            try {
              await onAdd(adv);
              setShowAdd(false);
            } catch {
              alert("Failed to add advertiser.");
            }
          }}
        />
      )}

      <div className="flex-1 overflow-y-auto p-2">
        {advertisers.map((adv) => (
          <AdvertiserCard
            key={adv.id}
            adv={adv}
            isEditing={editingId === adv.id}
            onEdit={() =>
              setEditingId(editingId === adv.id ? null : adv.id)
            }
            onUpdate={async (updates) => {
              try {
                await onUpdate(adv.id, updates);
                setEditingId(null);
              } catch {
                alert("Failed to update advertiser.");
              }
            }}
            onDelete={async () => {
              try {
                await onDelete(adv.id);
              } catch {
                alert("Failed to delete advertiser.");
              }
            }}
          />
        ))}
      </div>
    </div>
  );
}

function AdvertiserCard({
  adv,
  isEditing,
  onEdit,
  onUpdate,
  onDelete,
}: {
  adv: Advertiser;
  isEditing: boolean;
  onEdit: () => void;
  onUpdate: (
    updates: Partial<{
      name: string;
      intent: string;
      sigma: number;
      bid_price: number;
    }>
  ) => Promise<void>;
  onDelete: () => void;
}) {
  const [name, setName] = useState(adv.name);
  const [sigma, setSigma] = useState(adv.sigma);
  const [bidPrice, setBidPrice] = useState(adv.bid_price);
  const [intent, setIntent] = useState(adv.intent);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setName(adv.name);
    setSigma(adv.sigma);
    setBidPrice(adv.bid_price);
    setIntent(adv.intent);
  }, [adv.name, adv.sigma, adv.bid_price, adv.intent]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const updates: Record<string, string | number> = {};
      if (name !== adv.name) updates.name = name;
      if (intent !== adv.intent) updates.intent = intent;
      if (sigma !== adv.sigma) updates.sigma = sigma;
      if (bidPrice !== adv.bid_price) updates.bid_price = bidPrice;
      await onUpdate(updates);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="border border-slate-200 rounded-lg px-3 py-2.5 mb-2 bg-white">
      <div className="flex justify-between items-baseline mb-1">
        <div className="font-semibold text-sm">{adv.name}</div>
        <div className="text-xs text-slate-500 font-mono">
          ${adv.bid_price.toFixed(2)} &middot; &sigma;={adv.sigma.toFixed(2)}
        </div>
      </div>
      <div className="text-xs text-slate-500 leading-snug mb-2 line-clamp-2">
        {adv.intent}
      </div>

      <div className="flex gap-2">
        <button
          className="px-2.5 py-0.5 rounded border border-slate-300 bg-white text-xs cursor-pointer"
          onClick={onEdit}
        >
          {isEditing ? "Cancel" : "Edit"}
        </button>
        <button
          className="px-2.5 py-0.5 rounded border border-red-300 bg-rose-50 text-red-600 text-xs cursor-pointer"
          onClick={onDelete}
        >
          Delete
        </button>
      </div>

      {isEditing && (
        <div className="mt-2.5 pt-2.5 border-t border-slate-200 flex flex-col gap-1.5">
          <label className="text-xs font-semibold text-slate-600">Name</label>
          <input
            className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <label className="text-xs font-semibold text-slate-600">Intent</label>
          <textarea
            className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none resize-y font-[inherit]"
            value={intent}
            onChange={(e) => setIntent(e.target.value)}
            rows={3}
          />
          <div className="flex flex-col gap-0.5">
            <label className="text-xs font-semibold text-slate-600">
              Sigma: {sigma.toFixed(2)}
            </label>
            <input
              type="range"
              min="0.1"
              max="2.0"
              step="0.05"
              value={sigma}
              onChange={(e) => setSigma(parseFloat(e.target.value))}
              className="w-full"
            />
          </div>
          <label className="text-xs font-semibold text-slate-600">Bid Price ($)</label>
          <input
            className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none"
            type="number"
            step="0.25"
            min="0.25"
            value={bidPrice}
            onChange={(e) => setBidPrice(parseFloat(e.target.value))}
          />
          <button
            className="px-3.5 py-1.5 rounded-md border-none bg-blue-600 text-white text-[13px] font-semibold cursor-pointer mt-1"
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      )}
    </div>
  );
}

function AddForm({
  onAdd,
}: {
  onAdd: (adv: {
    name: string;
    intent: string;
    sigma: number;
    bid_price: number;
    budget: number;
  }) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [intent, setIntent] = useState("");
  const [sigma, setSigma] = useState(0.5);
  const [bidPrice, setBidPrice] = useState(2.0);
  const [budget, setBudget] = useState(500);
  const [saving, setSaving] = useState(false);

  const handleSubmit = async () => {
    if (!name || !intent) return;
    setSaving(true);
    try {
      await onAdd({ name, intent, sigma, bid_price: bidPrice, budget });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="px-4 py-3 border-b border-slate-200 flex flex-col gap-1.5 bg-slate-50">
      <label className="text-xs font-semibold text-slate-600">Name</label>
      <input
        className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Business name"
      />
      <label className="text-xs font-semibold text-slate-600">Intent</label>
      <textarea
        className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none resize-y font-[inherit]"
        value={intent}
        onChange={(e) => setIntent(e.target.value)}
        placeholder="Describe your service..."
        rows={3}
      />
      <div className="flex flex-col gap-0.5">
        <label className="text-xs font-semibold text-slate-600">Sigma: {sigma.toFixed(2)}</label>
        <input
          type="range"
          min="0.1"
          max="2.0"
          step="0.05"
          value={sigma}
          onChange={(e) => setSigma(parseFloat(e.target.value))}
          className="w-full"
        />
      </div>
      <label className="text-xs font-semibold text-slate-600">Bid Price ($)</label>
      <input
        className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none"
        type="number"
        step="0.25"
        min="0.25"
        value={bidPrice}
        onChange={(e) => setBidPrice(parseFloat(e.target.value))}
      />
      <label className="text-xs font-semibold text-slate-600">Budget ($)</label>
      <input
        className="px-2.5 py-1.5 rounded-md border border-slate-300 text-[13px] outline-none"
        type="number"
        step="50"
        min="50"
        value={budget}
        onChange={(e) => setBudget(parseFloat(e.target.value))}
      />
      <button
        className="px-3.5 py-1.5 rounded-md border-none bg-blue-600 text-white text-[13px] font-semibold cursor-pointer mt-1"
        onClick={handleSubmit}
        disabled={saving}
      >
        {saving ? "Adding..." : "Add Advertiser"}
      </button>
    </div>
  );
}
