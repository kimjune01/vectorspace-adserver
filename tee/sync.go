package tee

import (
	"vectorspace/enclave"
	"vectorspace/platform"
)

// SyncFromPlatform pushes current positions and budgets from the platform
// registry/tracker into a TEE proxy. Used for initial sync on startup.
func SyncFromPlatform(proxy TEEProxyInterface, registry *platform.PositionRegistry, budgets *platform.BudgetTracker) {
	positions := registry.GetAll()

	posSnaps := make([]enclave.PositionSnapshot, len(positions))
	for i, pos := range positions {
		posSnaps[i] = enclave.PositionSnapshot{
			ID:        pos.ID,
			Name:      pos.Name,
			Embedding: pos.Embedding,
			Sigma:     pos.Sigma,
			BidPrice:  pos.BidPrice,
			Currency:  pos.Currency,
			URL:       pos.URL,
			BudgetID:  pos.BudgetID,
		}
	}
	proxy.SyncPositions(posSnaps)

	var budgetSnaps []enclave.BudgetSnapshot
	seen := make(map[string]bool)
	for _, pos := range positions {
		key := pos.BudgetKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		info := budgets.GetInfo(key)
		if info == nil {
			continue
		}
		budgetSnaps = append(budgetSnaps, enclave.BudgetSnapshot{
			AdvertiserID: key,
			Total:        info.Total,
			Spent:        info.Spent,
			Currency:     info.Currency,
		})
	}
	proxy.SyncBudgets(budgetSnaps)
}
