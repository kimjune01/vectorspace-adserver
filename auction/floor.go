package auction

import "math"

// BidMeetsFloor returns true if the bid price meets or exceeds the floor price.
// Uses rounding to 4 decimal places to avoid floating-point comparison issues.
func BidMeetsFloor(bidPrice, floorPrice float64) bool {
	scale := math.Pow(10, 4)
	rounded := math.Round(bidPrice*scale) / scale
	floorRounded := math.Round(floorPrice*scale) / scale
	return rounded >= floorRounded
}

// EnforceBidFloor filters bids based on floor price.
// Returns eligible bids and IDs of rejected bids.
func EnforceBidFloor(bids []CoreBid, floor float64) (eligible []CoreBid, rejectedBidIDs []string) {
	eligibleBids := make([]CoreBid, 0, len(bids))
	rejectedIDs := make([]string, 0)

	for _, bid := range bids {
		if BidMeetsFloor(bid.Price, floor) {
			eligibleBids = append(eligibleBids, bid)
		} else {
			rejectedIDs = append(rejectedIDs, bid.ID)
		}
	}

	return eligibleBids, rejectedIDs
}
