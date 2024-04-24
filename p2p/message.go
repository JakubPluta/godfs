package p2p

const (
	IncomingMessage = 0x1
	IncomingStream  = 0x2
)

// Message holds any arbitrary data that is being sent over the
// each transport between nodes in the network.
type RPC struct {
	Payload []byte
	From    string
	Stream  bool
}

func (m *RPC) String() string {
	return string(m.Payload)
}
