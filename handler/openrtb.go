package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"vectorspace/platform"
)

// OpenRTB 2.5 wire-format endpoint. The exchange accepts a standard
// BidRequest and answers with a standard BidResponse, so a stack that
// already speaks OpenRTB can integrate without new request plumbing.
//
// Two deliberate deviations from stock 2.5 semantics, both declared here
// and in bid.ext.vectorspace rather than hidden:
//   - Settlement is per-click VCG, not CPM: bid.price is the amount charged
//     if the placement is clicked (ext.vectorspace.settlement = "cpc-vcg"),
//     and imp.bidfloor is interpreted in the same per-click unit.
//   - Inventory is a conversational text slot; adm is a self-contained HTML
//     snippet whose click-through routes via GET /click, which is where the
//     charge fires. Rendering adm as-is settles correctly with no
//     VectorSpace-specific code on the caller's side.
//
// The query point is resolved in order of precedence:
//  1. ext.vectorspace.embedding (request-level, then imp[0]-level)
//  2. ext.vectorspace.intent    (request-level, then imp[0]-level)
//  3. site/app.content.keywords, then user.keywords — comma-separated per
//     ORTB 2.5; each keyword is embedded separately and the best-scoring
//     point wins, so σ = 0 imports match on any exact keyword in the list
//
// This is the interop path: the query reaches the host in plaintext, which
// is what OpenRTB carries. The private path is POST /ad-request (encrypted
// embedding, TEE-isolated). See: june.kim/keywords-are-tiny-circles

type ortbVectorspaceExt struct {
	Embedding []float64 `json:"embedding,omitempty"`
	Intent    string    `json:"intent,omitempty"`
}

type ortbExt struct {
	Vectorspace *ortbVectorspaceExt `json:"vectorspace,omitempty"`
}

type ortbContent struct {
	Keywords string `json:"keywords,omitempty"`
}

type ortbSite struct {
	ID        string       `json:"id,omitempty"`
	Content   *ortbContent `json:"content,omitempty"`
	Publisher *struct {
		ID string `json:"id,omitempty"`
	} `json:"publisher,omitempty"`
}

type ortbUser struct {
	Keywords string `json:"keywords,omitempty"`
}

type ortbImp struct {
	ID       string   `json:"id"`
	BidFloor float64  `json:"bidfloor,omitempty"`
	Ext      *ortbExt `json:"ext,omitempty"`
}

type ortbBidRequest struct {
	ID   string    `json:"id"`
	Imp  []ortbImp `json:"imp"`
	Site *ortbSite `json:"site,omitempty"`
	App  *ortbSite `json:"app,omitempty"`
	User *ortbUser `json:"user,omitempty"`
	Cur  []string  `json:"cur,omitempty"`
	Test int       `json:"test,omitempty"`
	Ext  *ortbExt  `json:"ext,omitempty"`
}

type ortbBid struct {
	ID      string                 `json:"id"`
	ImpID   string                 `json:"impid"`
	Price   float64                `json:"price"`
	AdID    string                 `json:"adid,omitempty"`
	AdM     string                 `json:"adm,omitempty"`
	ADomain []string               `json:"adomain,omitempty"`
	Ext     map[string]interface{} `json:"ext,omitempty"`
}

type ortbSeatBid struct {
	Bid  []ortbBid `json:"bid"`
	Seat string    `json:"seat,omitempty"`
}

type ortbBidResponse struct {
	ID      string        `json:"id"`
	SeatBid []ortbSeatBid `json:"seatbid"`
	Cur     string        `json:"cur,omitempty"`
}

// OpenRTBHandler handles POST /openrtb2/auction.
type OpenRTBHandler struct {
	Engine *platform.AuctionEngine
}

// splitKeywords splits an ORTB comma-separated keywords field into trimmed,
// deduplicated, non-empty entries.
func splitKeywords(s string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, kw := range strings.Split(s, ",") {
		kw = strings.TrimSpace(kw)
		if kw == "" || seen[kw] {
			continue
		}
		seen[kw] = true
		out = append(out, kw)
	}
	return out
}

// resolveQuery extracts the query from a bid request, following the
// precedence order documented above.
func resolveQuery(req *ortbBidRequest) (texts []string, embedding []float64) {
	exts := []*ortbExt{req.Ext}
	if len(req.Imp) > 0 {
		exts = append(exts, req.Imp[0].Ext)
	}
	for _, ext := range exts {
		if ext != nil && ext.Vectorspace != nil && len(ext.Vectorspace.Embedding) > 0 {
			return nil, ext.Vectorspace.Embedding
		}
	}
	for _, ext := range exts {
		if ext != nil && ext.Vectorspace != nil && ext.Vectorspace.Intent != "" {
			return []string{ext.Vectorspace.Intent}, nil
		}
	}
	for _, s := range []*ortbSite{req.Site, req.App} {
		if s != nil && s.Content != nil && s.Content.Keywords != "" {
			if kws := splitKeywords(s.Content.Keywords); len(kws) > 0 {
				return kws, nil
			}
		}
	}
	if req.User != nil && req.User.Keywords != "" {
		if kws := splitKeywords(req.User.Keywords); len(kws) > 0 {
			return kws, nil
		}
	}
	return nil, nil
}

// clickBase reconstructs this server's external base URL for the adm
// click-through, honoring a reverse proxy's forwarded scheme. Both header
// values are request input, so they are validated before being embedded in
// markup: the scheme must be literally http or https, and the host must
// survive a URL round-trip.
func clickBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd == "http" || fwd == "https" {
		scheme = fwd
	}
	u, err := url.Parse(scheme + "://" + r.Host)
	if err != nil || u.Host == "" || u.Host != r.Host {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// HandleAuction handles POST /openrtb2/auction.
func (h *OpenRTBHandler) HandleAuction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ortbBidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "BidRequest.id is required", http.StatusBadRequest)
		return
	}
	if len(req.Imp) == 0 || req.Imp[0].ID == "" {
		http.Error(w, "BidRequest.imp with imp.id is required", http.StatusBadRequest)
		return
	}

	queryTexts, queryEmbedding := resolveQuery(&req)
	if len(queryTexts) == 0 && len(queryEmbedding) == 0 {
		http.Error(w, "no query point: set content.keywords, user.keywords, or ext.vectorspace", http.StatusBadRequest)
		return
	}

	publisherID := ""
	if req.Site != nil && req.Site.Publisher != nil {
		publisherID = req.Site.Publisher.ID
	}
	if publisherID == "" && req.App != nil && req.App.Publisher != nil {
		publisherID = req.App.Publisher.ID
	}

	result, err := h.Engine.RunORTBAuction(queryTexts, queryEmbedding, req.Imp[0].BidFloor, publisherID, req.Cur, req.Test == 1)
	if errors.Is(err, platform.ErrNoBid) {
		// ORTB convention: no bid is 204, not an error body.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		// Operational failure (embedder down, DB error) must not
		// masquerade as a quiet no-bid.
		http.Error(w, "auction failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Self-contained creative: rendering this HTML settles the click via
	// GET /click, so a standard renderer needs no VectorSpace-specific code.
	// Test requests are never logged, so their creative links straight to
	// the advertiser (renderable, never billable).
	label := result.AdTitle
	if label == "" {
		label = result.Winner.Name
	}
	clickURL := ""
	if result.AuctionID > 0 {
		if base := clickBase(r); base != "" {
			clickURL = fmt.Sprintf("%s/click?auction_id=%d", base, result.AuctionID)
		}
	} else if req.Test == 1 {
		clickURL = result.ClickURL
	}
	adm := ""
	if clickURL != "" {
		adm = fmt.Sprintf(`<a href="%s" rel="nofollow sponsored">%s</a>`, html.EscapeString(clickURL), html.EscapeString(label))
		if result.AdSubtitle != "" {
			adm += fmt.Sprintf(` <span>%s</span>`, html.EscapeString(result.AdSubtitle))
		}
	}

	var adomain []string
	if result.ClickURL != "" {
		if u, err := url.Parse(result.ClickURL); err == nil && u.Host != "" {
			adomain = []string{u.Host}
		}
	}

	resp := ortbBidResponse{
		ID:  req.ID,
		Cur: result.Currency,
		SeatBid: []ortbSeatBid{{
			Seat: result.Winner.ID,
			Bid: []ortbBid{{
				ID:      result.Winner.ID,
				ImpID:   req.Imp[0].ID,
				Price:   result.Payment,
				AdID:    result.Winner.ID,
				AdM:     adm,
				ADomain: adomain,
				Ext: map[string]interface{}{
					"vectorspace": map[string]interface{}{
						"auction_id":  result.AuctionID,
						"advertiser":  result.Winner.Name,
						"sigma":       result.Winner.Sigma,
						"score":       result.Winner.Score,
						"click_url":   result.ClickURL,
						"ad_title":    result.AdTitle,
						"ad_subtitle": result.AdSubtitle,
						// Settlement is per-click VCG, not CPM: price is what
						// the winner pays if the placement is clicked.
						"settlement": "cpc-vcg",
					},
				},
			}},
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
