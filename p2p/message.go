package p2p

// Message holds any arbitrary data that is being sent over the
// each transport between nodes in the network.
type RPC struct {
	Payload []byte
	From    string
}

func (m *RPC) String() string {
	return string(m.Payload)
}
