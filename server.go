package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/JakubPluta/godfs/p2p"
)

type FileServerOpts struct {
	ID                string
	EncKey            []byte
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	BootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts

	peerLock sync.Mutex
	peers    map[string]p2p.Peer

	store    *Store
	quitchan chan struct{}
}

func generateID() string {
	buf := make([]byte, 32)
	io.ReadFull(rand.Reader, buf)
	return hex.EncodeToString(buf)
}

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	if len(opts.ID) == 0 {
		opts.ID = generateID()
	}
	return &FileServer{
		store:          NewStore(storeOpts),
		FileServerOpts: opts,
		quitchan:       make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

func (f *FileServer) OnPeer(peer p2p.Peer) error {
	f.peerLock.Lock()
	defer f.peerLock.Unlock()
	f.peers[peer.RemoteAddr().String()] = peer
	log.Printf("new peer connected: %s", peer.RemoteAddr().String())
	return nil
}

func (f *FileServer) Stop() {
	close(f.quitchan)
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	ID   string
	Key  string
	Size int64
}

// Here we send just normal message, no streaming
func (f *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range f.peers {
		// we are going to send incoming streaming type
		peer.Send([]byte{p2p.IncomingMessage})

		if err := peer.Send(buf.Bytes()); err != nil {
			log.Println("failed to send key: ", err)
			return err
		}
	}
	return nil
}

type MessageGetFile struct {
	ID  string
	Key string
}

func (f *FileServer) Get(key string) (io.Reader, error) {
	if f.store.Has(f.ID, key) {
		fmt.Printf("[%s] serving file (%s) from disk\n", f.Transport.Addr(), key)
		_, r, err := f.store.Read(f.ID, key)
		return r, err
	}
	fmt.Printf("[%s] dont have file (%s) locally, fetching from network...\n", f.Transport.Addr(), key)
	msg := Message{
		Payload: MessageGetFile{Key: key, ID: f.ID},
	}
	if err := f.broadcast(&msg); err != nil {
		return nil, err
	}
	time.Sleep(time.Millisecond * 5)
	for _, peer := range f.peers {
		// First read the file size so we can limit the amount of bytes
		// that we read from connection, so it will not hanging
		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize)

		n, err := f.store.WriteDecrypt(f.EncKey, key, f.ID, io.LimitReader(peer, fileSize))

		if err != nil {
			return nil, err
		}

		fmt.Printf("[%s] received (%d) bytes over the network from (%s)\n", f.Transport.Addr(), n, peer.RemoteAddr())

		peer.CloseStream()
	}
	_, r, err := f.store.Read(f.ID, key)
	return r, err
}

func (f *FileServer) Store(key string, r io.Reader) error {

	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := f.store.Write(f.ID, key, tee)
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{ID: f.ID, Key: key, Size: size + 16}, // 16 byters to iv encryption
	}
	if err := f.broadcast(&msg); err != nil {
		return err
	}
	time.Sleep(time.Millisecond * 5)
	peers := []io.Writer{}

	for _, peer := range f.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)
	mw.Write([]byte{p2p.IncomingStream})
	n, err := copyEncrypt(f.EncKey, fileBuffer, mw)
	if err != nil {
		log.Println("failed to send key: ", err)
		return err
	}
	fmt.Printf("[%s] received and written (%d) bytes to the disk\n", f.Transport.Addr(), n)
	return nil
}

func (f *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to error or user quit action")
		f.Transport.Close()
	}()
	for {
		select {
		case rpc := <-f.Transport.Consume():
			var msg Message

			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("failed to decode payload: ", err, string(msg.Payload.([]byte)))

			}

			if err := f.handleMessage(rpc.From, &msg); err != nil {
				log.Println("failed to handle message: ", err)

			}

		case <-f.quitchan:
			return
		}
	}
}

func (fs *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return fs.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return fs.handleMessageGetFile(from, v)
	}
	return nil
}

func (fs *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !fs.store.Has(msg.ID, msg.Key) {
		return fmt.Errorf("[%s] need to server file (%s) but it was not found locally", fs.Transport.Addr(), msg.Key)
	}
	fmt.Printf("[%s] serving file (%s) over the network\n", fs.Transport.Addr(), msg.Key)

	fileSize, r, err := fs.store.Read(msg.ID, msg.Key)
	if err != nil {
		return err
	}

	if rc, ok := r.(io.ReadCloser); ok {
		fmt.Println("closing readCloser")
		defer rc.Close()
	}

	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("peer not found (%s) in peer list", from)
	}
	// first send `IncomingStream` to peer
	// and then send the file size as an int64
	peer.Send([]byte{p2p.IncomingStream})
	binary.Write(peer, binary.LittleEndian, fileSize)

	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}
	fmt.Printf("[%s] written %d bytes over the network to %s \n", fs.Transport.Addr(), n, from)
	return nil

}

func (fs *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("peer not found (%s) in peer list", from)
	}

	n, err := fs.store.Write(msg.ID, msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}
	fmt.Printf("[%s] received and written %d bytes to disk\n", fs.Transport.Addr(), n)
	peer.CloseStream()
	return nil
}

func (fs *FileServer) bootstrapNetwork() error {
	for _, addr := range fs.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		fmt.Printf("[%s] attempting to connect with remote node: %s\n", fs.Transport.Addr(), addr)
		go func(addr string) {
			if err := fs.Transport.Dial(addr); err != nil {
				log.Println("dial error: ", err)
			}
		}(addr)
	}
	return nil
}

func (fs *FileServer) Start() error {
	fmt.Printf("[%s] starting fileserver\n", fs.Transport.Addr())
	if err := fs.Transport.ListenAndAccept(); err != nil {
		return err
	}

	fs.bootstrapNetwork()

	fs.loop()

	return nil
}

func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}
