package auction

// EnforceBidFloor separates bids into those meeting the floor and those rejected.
// Floor is a minimum price threshold set by the publisher.
// See: june.kim/three-levers (τ as relevance gate)
func EnforceBidFloor(bids []CoreBid, floor float64) (eligible []CoreBid, rejectedIDs []string) {
	for _, bid := range bids {
		if bid.Price >= floor {
			eligible = append(eligible, bid)
		} else {
			rejectedIDs = append(rejectedIDs, bid.ID)
		}
	}
	return
}
