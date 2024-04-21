package p2p

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

// TCPPeer represents the remote node over TCP established connection.
type TCPPeer struct {
	// underlying connection of the peer
	conn net.Conn

	// if we dial a connection, we are outbound. So outboud == true
	// if we accept a connection, we are inbound. So outbound == false
	outboud bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:    conn,
		outboud: outbound,
	}
}

// Close implements the Peer interface.
func (t *TCPPeer) Close() error {
	return t.conn.Close()
}

// RemoteAddr implements the Peer interface.
// It returns the address of the underlying connection of remote peer.
func (t *TCPPeer) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

type TCPTransportOpts struct {
	ListenAddr    string
	HandshakeFunc HandshakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcchan  chan RPC
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcchan:          make(chan RPC),
	}
}

// Consume implements the Transport interface, which will return read-only channel.
// for reading the incoming messages received from other peer in the network.
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcchan
}

// Close implements the Transport interface.
func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

// Dial implements the Transport interface.
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	// peer := NewTCPPeer(conn, false)
	// if err = t.HandshakeFunc(peer); err != nil {
	// 	return err
	// }
	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}
	go t.startAcceptLoop()
	log.Printf("TCPTransport: Listening on %s\n", t.ListenAddr)
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			// TODO
			fmt.Printf("TCPTransport: Error accepting connection: %v\n", err)
		}
		// As we accept a connection, we are
		go t.handleConn(conn, false)

	}
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error

	defer func() {
		fmt.Printf("dropping peer connection: %v\n", err)
		conn.Close()

	}()

	peer := NewTCPPeer(conn, outbound)
	if err = t.HandshakeFunc(peer); err != nil {
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	rpc := RPC{}
	for {
		err = t.Decoder.Decode(conn, &rpc)
		if err == io.EOF {
			fmt.Printf("TCPTransport: Connection closed: %v\n", err)
			return
		}

		if err != nil {
			fmt.Printf("TCPTransport: Error during decoding: %v\n", err)
			continue

		}
		rpc.From = conn.RemoteAddr()
		t.rpcchan <- rpc
		fmt.Printf("TCPTransport: Received message: %v\n", rpc)
	}

}
