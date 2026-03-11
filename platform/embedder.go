package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Embedder is an HTTP client for embedding text via the sidecar or Hugging Face Inference API.
type Embedder struct {
	baseURL    string
	hfToken    string
	httpClient *http.Client
}

func NewEmbedder(sidecarURL string) *Embedder {
	return &Embedder{
		baseURL: sidecarURL,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 128,
				IdleConnTimeout:    90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
	}
}

// NewHuggingFaceEmbedder creates an embedder that calls the Hugging Face Inference API.
func NewHuggingFaceEmbedder(model, token string) *Embedder {
	if model == "" {
		model = "BAAI/bge-small-en-v1.5"
	}
	return &Embedder{
		baseURL: "https://router.huggingface.co/hf-inference/models/" + model + "/pipeline/feature-extraction",
		hfToken: token,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 128,
				IdleConnTimeout:    90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
	}
}

type embedRequest struct {
	Text  string   `json:"text,omitempty"`
	Texts []string `json:"texts,omitempty"`
}

type embedResponse struct {
	Embedding  []float64   `json:"embedding,omitempty"`
	Embeddings [][]float64 `json:"embeddings,omitempty"`
	Dim        int         `json:"dim"`
}

// Embed returns the embedding vector for a single text string.
func (e *Embedder) Embed(text string) ([]float64, error) {
	if e.hfToken != "" {
		return e.hfEmbed(text)
	}
	return e.sidecarEmbed(text)
}

// EmbedBatch returns embedding vectors for multiple texts in one call.
func (e *Embedder) EmbedBatch(texts []string) ([][]float64, error) {
	if e.hfToken != "" {
		return e.hfEmbedBatch(texts)
	}
	return e.sidecarEmbedBatch(texts)
}

// --- sidecar backend ---

func (e *Embedder) sidecarEmbed(text string) ([]float64, error) {
	body, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	resp, err := e.httpClient.Post(e.baseURL+"/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sidecar request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sidecar response: %w", err)
	}

	return result.Embedding, nil
}

func (e *Embedder) sidecarEmbedBatch(texts []string) ([][]float64, error) {
	body, err := json.Marshal(embedRequest{Texts: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	resp, err := e.httpClient.Post(e.baseURL+"/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sidecar request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sidecar response: %w", err)
	}

	return result.Embeddings, nil
}

// --- Hugging Face backend ---

type hfRequest struct {
	Inputs    interface{} `json:"inputs"`
	Normalize bool        `json:"normalize"`
	Truncate  bool        `json:"truncate"`
}

func (e *Embedder) hfPost(body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", e.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.hfToken)
	return e.httpClient.Do(req)
}

func (e *Embedder) hfEmbed(text string) ([]float64, error) {
	body, err := json.Marshal(hfRequest{Inputs: text, Normalize: true, Truncate: true})
	if err != nil {
		return nil, fmt.Errorf("marshal hf request: %w", err)
	}

	resp, err := e.hfPost(body)
	if err != nil {
		return nil, fmt.Errorf("hf request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hf returned status %d", resp.StatusCode)
	}

	// HF returns a flat array for single input: [0.1, 0.2, ...]
	var result []float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode hf response: %w", err)
	}

	return result, nil
}

func (e *Embedder) hfEmbedBatch(texts []string) ([][]float64, error) {
	body, err := json.Marshal(hfRequest{Inputs: texts, Normalize: true, Truncate: true})
	if err != nil {
		return nil, fmt.Errorf("marshal hf request: %w", err)
	}

	resp, err := e.hfPost(body)
	if err != nil {
		return nil, fmt.Errorf("hf request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hf returned status %d", resp.StatusCode)
	}

	// HF returns array of arrays for batch: [[0.1, 0.2, ...], [0.3, 0.4, ...]]
	var result [][]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode hf response: %w", err)
	}

	return result, nil
}
