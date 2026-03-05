export interface BidderDetail {
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

export interface AuctionResult {
  intent: string;
  winner: BidderDetail | null;
  runner_up: BidderDetail | null;
  all_bidders: BidderDetail[];
  payment: number;
  currency: string;
  bid_count: number;
  eligible_count: number;
}

export interface Advertiser {
  id: string;
  name: string;
  intent: string;
  sigma: number;
  bid_price: number;
  currency: string;
}

export interface BudgetInfo {
  advertiser_id: string;
  total: number;
  spent: number;
  remaining: number;
  currency: string;
}

export interface Stats {
  auction_count: number;
  total_spend: number;
  cloudx_revenue: number;
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface PrebuiltConversation {
  label: string;
  vertical: string;
  variant: "generic" | "specific";
  messages: ChatMessage[];
}
