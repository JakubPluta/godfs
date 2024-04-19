package p2p

import (
	"fmt"
	"net"
	"sync"
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

type TCPTransportOpts struct {
	ListenAddr    string
	HandshakeFunc HandshakeFunc
	Decoder       Decoder
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener

	mu    sync.RWMutex
	peers map[net.Addr]Peer
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
	}
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}
	go t.startAcceptLoop()
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			// TODO
			fmt.Printf("TCPTransport: Error accepting connection: %v\n", err)
		}
		// As we accept a connection, we are
		go t.handleConn(conn)

	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	peer := NewTCPPeer(conn, true)

	if err := t.HandshakeFunc(peer); err != nil {
		// TODO
		fmt.Printf("TCPTransport: Error during handshake: %v\n", err)
		conn.Close()
		return
	}

	rpc := &RPC{}
	for {
		if err := t.Decoder.Decode(conn, rpc); err != nil {
			fmt.Printf("TCPTransport: Error during decoding: %v\n", err)
			continue

		}
		rpc.From = conn.RemoteAddr()
		fmt.Printf("TCPTransport: Received message: %v\n", rpc)
	}

}
