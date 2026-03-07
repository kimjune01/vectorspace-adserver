package tee

import (
	"cloudx-adserver/enclave"
	"cloudx-adserver/platform"
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
		}
	}
	proxy.SyncPositions(posSnaps)

	var budgetSnaps []enclave.BudgetSnapshot
	for _, pos := range positions {
		info := budgets.GetInfo(pos.ID)
		if info == nil {
			continue
		}
		budgetSnaps = append(budgetSnaps, enclave.BudgetSnapshot{
			AdvertiserID: pos.ID,
			Total:        info.Total,
			Spent:        info.Spent,
			Currency:     info.Currency,
		})
	}
	proxy.SyncBudgets(budgetSnaps)
}
