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
    <div style={styles.container}>
      <div style={styles.header}>
        <h3 style={styles.title}>Advertiser Roster</h3>
        <button style={styles.addBtn} onClick={() => setShowAdd(!showAdd)}>
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

      <div style={styles.list}>
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

  // Sync local state when props change (e.g. after another card's edit triggers refetch)
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
    <div style={styles.card}>
      <div style={styles.cardHeader}>
        <div style={styles.cardName}>{adv.name}</div>
        <div style={styles.cardMeta}>
          ${adv.bid_price.toFixed(2)} &middot; &sigma;={adv.sigma.toFixed(2)}
        </div>
      </div>
      <div style={styles.cardIntent}>{adv.intent}</div>

      <div style={styles.cardActions}>
        <button style={styles.editBtn} onClick={onEdit}>
          {isEditing ? "Cancel" : "Edit"}
        </button>
        <button style={styles.deleteBtn} onClick={onDelete}>
          Delete
        </button>
      </div>

      {isEditing && (
        <div style={styles.editForm}>
          <label style={styles.fieldLabel}>Name</label>
          <input
            style={styles.fieldInput}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <label style={styles.fieldLabel}>Intent</label>
          <textarea
            style={styles.fieldTextarea}
            value={intent}
            onChange={(e) => setIntent(e.target.value)}
            rows={3}
          />
          <div style={styles.sliderRow}>
            <label style={styles.fieldLabel}>
              Sigma: {sigma.toFixed(2)}
            </label>
            <input
              type="range"
              min="0.1"
              max="2.0"
              step="0.05"
              value={sigma}
              onChange={(e) => setSigma(parseFloat(e.target.value))}
              style={styles.slider}
            />
          </div>
          <label style={styles.fieldLabel}>Bid Price ($)</label>
          <input
            style={styles.fieldInput}
            type="number"
            step="0.25"
            min="0.25"
            value={bidPrice}
            onChange={(e) => setBidPrice(parseFloat(e.target.value))}
          />
          <button
            style={styles.saveBtn}
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
    <div style={styles.addForm}>
      <label style={styles.fieldLabel}>Name</label>
      <input
        style={styles.fieldInput}
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Business name"
      />
      <label style={styles.fieldLabel}>Intent</label>
      <textarea
        style={styles.fieldTextarea}
        value={intent}
        onChange={(e) => setIntent(e.target.value)}
        placeholder="Describe your service..."
        rows={3}
      />
      <div style={styles.sliderRow}>
        <label style={styles.fieldLabel}>Sigma: {sigma.toFixed(2)}</label>
        <input
          type="range"
          min="0.1"
          max="2.0"
          step="0.05"
          value={sigma}
          onChange={(e) => setSigma(parseFloat(e.target.value))}
          style={styles.slider}
        />
      </div>
      <label style={styles.fieldLabel}>Bid Price ($)</label>
      <input
        style={styles.fieldInput}
        type="number"
        step="0.25"
        min="0.25"
        value={bidPrice}
        onChange={(e) => setBidPrice(parseFloat(e.target.value))}
      />
      <label style={styles.fieldLabel}>Budget ($)</label>
      <input
        style={styles.fieldInput}
        type="number"
        step="50"
        min="50"
        value={budget}
        onChange={(e) => setBudget(parseFloat(e.target.value))}
      />
      <button style={styles.saveBtn} onClick={handleSubmit} disabled={saving}>
        {saving ? "Adding..." : "Add Advertiser"}
      </button>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: "flex",
    flexDirection: "column",
    height: "100%",
    overflow: "hidden",
  },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    padding: "12px 16px",
    borderBottom: "1px solid #e2e8f0",
  },
  title: { margin: 0, fontSize: "16px" },
  addBtn: {
    padding: "4px 12px",
    borderRadius: "6px",
    border: "1px solid #cbd5e1",
    background: "white",
    fontSize: "13px",
    cursor: "pointer",
  },
  list: {
    flex: 1,
    overflowY: "auto",
    padding: "8px",
  },
  card: {
    border: "1px solid #e2e8f0",
    borderRadius: "8px",
    padding: "10px 12px",
    marginBottom: "8px",
    background: "white",
  },
  cardHeader: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "baseline",
    marginBottom: "4px",
  },
  cardName: { fontWeight: 600, fontSize: "14px" },
  cardMeta: { fontSize: "12px", color: "#64748b", fontFamily: "monospace" },
  cardIntent: {
    fontSize: "12px",
    color: "#64748b",
    lineHeight: "1.4",
    marginBottom: "8px",
    display: "-webkit-box",
    WebkitLineClamp: 2,
    WebkitBoxOrient: "vertical",
    overflow: "hidden",
  },
  cardActions: { display: "flex", gap: "8px" },
  editBtn: {
    padding: "3px 10px",
    borderRadius: "4px",
    border: "1px solid #cbd5e1",
    background: "white",
    fontSize: "12px",
    cursor: "pointer",
  },
  deleteBtn: {
    padding: "3px 10px",
    borderRadius: "4px",
    border: "1px solid #fca5a5",
    background: "#fff1f2",
    color: "#dc2626",
    fontSize: "12px",
    cursor: "pointer",
  },
  editForm: {
    marginTop: "10px",
    paddingTop: "10px",
    borderTop: "1px solid #e2e8f0",
    display: "flex",
    flexDirection: "column",
    gap: "6px",
  },
  addForm: {
    padding: "12px 16px",
    borderBottom: "1px solid #e2e8f0",
    display: "flex",
    flexDirection: "column",
    gap: "6px",
    background: "#f8fafc",
  },
  fieldLabel: { fontSize: "12px", fontWeight: 600, color: "#475569" },
  fieldInput: {
    padding: "6px 10px",
    borderRadius: "6px",
    border: "1px solid #cbd5e1",
    fontSize: "13px",
    outline: "none",
  },
  fieldTextarea: {
    padding: "6px 10px",
    borderRadius: "6px",
    border: "1px solid #cbd5e1",
    fontSize: "13px",
    outline: "none",
    resize: "vertical",
    fontFamily: "inherit",
  },
  sliderRow: {
    display: "flex",
    flexDirection: "column",
    gap: "2px",
  },
  slider: { width: "100%" },
  saveBtn: {
    padding: "6px 14px",
    borderRadius: "6px",
    border: "none",
    background: "#2563eb",
    color: "white",
    fontSize: "13px",
    fontWeight: 600,
    cursor: "pointer",
    marginTop: "4px",
  },
};
