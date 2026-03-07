export interface AuctionRow {
  id: number;
  intent: string;
  winner_id: string;
  winner_name: string;
  payment: number;
  currency: string;
  bid_count: number;
  created_at: string;
}

export interface RevenuePeriod {
  period: string;
  auction_count: number;
  total_spend: number;
  publisher_revenue: number;
  exchange_revenue: number;
}

export interface AdvertiserSpend {
  advertiser_id: string;
  name: string;
  total_spend: number;
  auction_count: number;
}

export interface AdvertiserWithBudget {
  id: string;
  name: string;
  intent: string;
  sigma: number;
  bid_price: number;
  budget_total: number;
  budget_spent: number;
  currency: string;
  url: string;
}

export interface EventStats {
  impressions: number;
  clicks: number;
  viewable: number;
}

export interface PortalProfile {
  id: string;
  name: string;
  intent: string;
  sigma: number;
  bid_price: number;
  currency: string;
  budget_total: number;
  budget_spent: number;
  budget_remaining: number;
  url: string;
}

export interface PublisherInfo {
  id: string;
  name: string;
  domain: string;
  created_at: string;
}

export interface PublisherProfile {
  id: string;
  name: string;
  domain: string;
}

export interface PublisherStats {
  auction_count: number;
  total_revenue: number;
  currency: string;
}

export interface SimulationBidder {
  id: string;
  rank: number;
  name: string;
  intent: string;
  bid_price: number;
  sigma: number;
  score: number;
  distance_sq: number;
  log_bid: number;
}

export interface TauBucket {
  tau: number;
  count: number;
}

export interface SimulationResult {
  intent: string;
  winner: SimulationBidder | null;
  all_bidders: SimulationBidder[];
  payment: number;
  bid_count: number;
  tau_thresholds: TauBucket[];
}
