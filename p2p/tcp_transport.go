package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// TCPPeer represents the remote node over TCP established connection.
type TCPPeer struct {
	// The underlying connection of the peer. Which is an TCP connection
	net.Conn

	// if we dial a connection, we are outbound. So outboud == true
	// if we accept a connection, we are inbound. So outbound == false
	outboud bool

	wg *sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:    conn,
		outboud: outbound,
		wg:      &sync.WaitGroup{},
	}
}

func (t *TCPPeer) CloseStream() {
	t.wg.Done()
}

// Send implements the Peer interface.
func (t *TCPPeer) Send(payload []byte) error {
	_, err := t.Conn.Write(payload)
	return err
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
		rpcchan:          make(chan RPC, 1024),
	}
}

// Consume implements the Transport interface, which will return read-only channel.
// for reading the incoming messages received from other peer in the network.
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcchan
}

// Addr implements the Transport interface.
// It returns the listen address of the TCPTransport.
func (t *TCPTransport) Addr() string {
	return t.ListenAddr
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

	for {
		rpc := RPC{}
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			return
		}
		rpc.From = conn.RemoteAddr().String()
		if rpc.Stream {
			peer.wg.Add(1)
			fmt.Printf("[%s] incoming stream, waiting till stream is done\n", rpc.From)
			peer.wg.Wait()
			fmt.Printf("[%s] stream closed, resume normal read\n", rpc.From)
			continue
		}

		t.rpcchan <- rpc

	}

}
