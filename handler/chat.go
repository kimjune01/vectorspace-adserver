package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages []chatMessage `json:"messages"`
	System   string        `json:"system,omitempty"`
}

type chatResponse struct {
	Content string `json:"content"`
}

// anthropicRequest is the Anthropic Messages API request format.
type anthropicRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []chatMessage `json:"messages"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

// ChatHandler proxies chat requests to the Anthropic API.
type ChatHandler struct {
	APIKey string
}

// HandleChat handles POST /chat
func (h *ChatHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.APIKey == "" {
		http.Error(w, "Anthropic API key not configured", http.StatusServiceUnavailable)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, "messages is required", http.StatusBadRequest)
		return
	}

	// Ensure conversation ends with a user message (required by newer models)
	msgs := req.Messages
	if msgs[len(msgs)-1].Role == "assistant" {
		msgs = append(msgs, chatMessage{Role: "user", Content: "Based on the conversation above, please respond."})
	}

	// Build Anthropic API request
	apiReq := anthropicRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  msgs,
	}
	if req.System != "" {
		apiReq.System = req.System
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		http.Error(w, "failed to marshal request", http.StatusInternalServerError)
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", h.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		http.Error(w, "Anthropic API request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read API response", http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Anthropic API error (status %d): %s", resp.StatusCode, string(respBody)), http.StatusBadGateway)
		return
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		http.Error(w, "failed to parse API response", http.StatusBadGateway)
		return
	}

	// Extract text content
	content := ""
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{Content: content})
}
