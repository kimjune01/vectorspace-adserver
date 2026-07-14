//go:build !nitro

package enclave

import (
	"fmt"
	"net"
)

// ListenVsock falls back to TCP on localhost for dev and test outside an
// enclave, where AF_VSOCK is unavailable. The `nitro` build uses real vsock
// (listen_nitro.go). This fallback must never run in production.
func ListenVsock(port int) (net.Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return net.Listen("tcp", addr)
}
