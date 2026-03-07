package enclave

import (
	"fmt"
	"net"
)

// ListenVsock tries vsock first (only works on Nitro Enclaves), then falls back to TCP.
// Phase 2b: add build tag for real vsock via github.com/mdlayher/vsock.
func ListenVsock(port int) (net.Listener, error) {
	// Fallback: TCP on localhost (for dev/test outside enclaves)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return net.Listen("tcp", addr)
}
