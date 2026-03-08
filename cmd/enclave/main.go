// Command enclave runs inside a Nitro Enclave. It listens on vsock port 5000
// and processes auction requests, key requests, and sync messages from the parent.
//
// Build into an EIF via:
//
//	docker build -t vectorspace-enclave .
//	nitro-cli build-enclave --docker-uri vectorspace-enclave:latest --output-file vectorspace-enclave.eif
package main

import (
	"vectorspace/enclave"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
)

const defaultPort = 5000

// message is the envelope for all vsock messages.
type message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// response wraps a typed response back to the parent.
type response struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func main() {
	log.SetOutput(os.Stderr)
	log.Println("enclave: starting")

	km, err := enclave.NewKeyManager()
	if err != nil {
		log.Fatalf("enclave: keygen: %v", err)
	}
	log.Println("enclave: RSA-2048 keypair generated")

	positions := enclave.NewPositionStore()
	budgets := enclave.NewBudgetStore()

	// Listen on vsock. On a real Nitro Enclave this uses AF_VSOCK.
	// For local testing, fall back to TCP.
	listener, err := enclave.ListenVsock(defaultPort)
	if err != nil {
		log.Fatalf("enclave: listen: %v", err)
	}
	log.Printf("enclave: listening on port %d", defaultPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("enclave: accept: %v", err)
			continue
		}
		go handleConn(conn, km, positions, budgets)
	}
}

func handleConn(conn net.Conn, km *enclave.KeyManager, positions *enclave.PositionStore, budgets *enclave.BudgetStore) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var msg message
	if err := dec.Decode(&msg); err != nil {
		enc.Encode(response{Type: "error", Error: fmt.Sprintf("decode: %v", err)})
		return
	}

	switch msg.Type {
	case "ping":
		enc.Encode(response{Type: "pong"})

	case "key_request":
		enc.Encode(response{
			Type: "key_response",
			Payload: enclave.AttestationResponse{
				PublicKey:      km.PublicKeyPEM(),
				AttestationB64: generateAttestation(km),
			},
		})

	case "sync_positions":
		var snap []enclave.PositionSnapshot
		if err := json.Unmarshal(msg.Payload, &snap); err != nil {
			enc.Encode(response{Type: "error", Error: fmt.Sprintf("unmarshal positions: %v", err)})
			return
		}
		positions.ReplaceAll(snap)
		enc.Encode(response{Type: "sync_ack"})

	case "sync_budgets":
		var snap []enclave.BudgetSnapshot
		if err := json.Unmarshal(msg.Payload, &snap); err != nil {
			enc.Encode(response{Type: "error", Error: fmt.Sprintf("unmarshal budgets: %v", err)})
			return
		}
		budgets.ReplaceAll(snap)
		enc.Encode(response{Type: "sync_ack"})

	case "auction_request":
		var req enclave.AuctionRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			enc.Encode(response{Type: "error", Error: fmt.Sprintf("unmarshal auction request: %v", err)})
			return
		}
		result, err := enclave.ProcessPrivateAuction(&req, km.PrivateKey(), positions, budgets)
		if err != nil {
			enc.Encode(response{Type: "error", Error: err.Error()})
			return
		}
		enc.Encode(response{Type: "auction_response", Payload: result})

	default:
		enc.Encode(response{Type: "error", Error: fmt.Sprintf("unknown message type: %s", msg.Type)})
	}
}

// generateAttestation returns a base64-encoded COSE attestation document.
// On a real Nitro Enclave, this calls the NSM device via github.com/hf/nsm.
// Outside an enclave (dev/test), returns a placeholder.
func generateAttestation(km *enclave.KeyManager) string {
	// Phase 2b: Use github.com/hf/nsm to generate real attestation:
	//
	//   sess, _ := nsm.OpenDefaultSession()
	//   defer sess.Close()
	//   res, _ := sess.Send(&request.Attestation{
	//       PublicKey: pubKeyDER,
	//   })
	//   return base64.StdEncoding.EncodeToString(res.Attestation.Document)
	return "placeholder-attestation-not-for-production"
}
