package p2p

import (
	"encoding/gob"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type GOBDecoder struct{}

func (d GOBDecoder) Decode(r io.Reader, msg *RPC) error {
	return gob.NewDecoder(r).Decode(msg)
}

type DefaultDecoder struct{}

func (d DefaultDecoder) Decode(r io.Reader, msg *RPC) error {

	peekBuf := make([]byte, 1)
	if _, err := r.Read(peekBuf); err != nil {
		return err
	}

	// In case of the incoming stream we don't need to decode
	// what is beeing sent over the network. We just set the Stream to true
	// so we can handle it later.
	stream := peekBuf[0] == IncomingStream
	if stream {
		msg.Stream = true
		return nil
	}

	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		return err
	}
	msg.Payload = buf[:n]
	return nil
}
