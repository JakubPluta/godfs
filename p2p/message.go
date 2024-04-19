package p2p

import "net"

// Message holds any arbitrary data that is being sent over the
// each transport between nodes in the network.
type RPC struct {
	Payload []byte
	From    net.Addr
}

func (m *RPC) String() string {
	return string(m.Payload)
}
