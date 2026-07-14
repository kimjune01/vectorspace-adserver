//go:build nitro

package enclave

import (
	"net"

	"github.com/mdlayher/vsock"
)

// ListenVsock listens on AF_VSOCK for connections from the parent instance.
// Inside a Nitro Enclave this is the only transport in or out; the enclave has
// no network interface. A nil config binds the enclave's own context ID
// (discovered automatically) at the given port; the parent connects to that CID
// and the same port.
func ListenVsock(port int) (net.Listener, error) {
	return vsock.Listen(uint32(port), nil)
}
