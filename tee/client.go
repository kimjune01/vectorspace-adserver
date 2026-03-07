package tee

import (
	"encoding/json"
	"fmt"
	"net"
)

// VsockClient connects to the enclave over vsock (or TCP for local dev).
type VsockClient struct {
	CID  uint32
	Port uint32
}

// message is the envelope for all vsock messages.
type message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// rawResponse is the envelope for vsock responses.
type rawResponse struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Send dials the enclave, sends a typed message, and returns the response.
// Phase 2b: use mdlayher/vsock.Dial(cid, port) instead of TCP.
// For now, falls back to TCP on localhost for dev/test.
func (c *VsockClient) Send(msgType string, payload interface{}) (*rawResponse, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("vsock dial: %w", err)
	}
	defer conn.Close()

	// Encode request
	msg := message{Type: msgType}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		msg.Payload = json.RawMessage(raw)
	}

	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Decode response
	var resp rawResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("enclave error: %s", resp.Error)
	}

	return &resp, nil
}

func (c *VsockClient) dial() (net.Conn, error) {
	// Phase 2b: use mdlayher/vsock for real Nitro Enclaves:
	//   return vsock.Dial(c.CID, c.Port, nil)
	//
	// For dev/test, fall back to TCP localhost.
	addr := fmt.Sprintf("127.0.0.1:%d", c.Port)
	return net.Dial("tcp", addr)
}
