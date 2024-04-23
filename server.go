package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/JakubPluta/godfs/p2p"
)

type FileServerOpts struct {
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

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
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
	Key  string
	Size int64
}

func (f *FileServer) broadcast(msg *Message) error {
	peers := []io.Writer{}
	for _, peer := range f.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)
	return gob.NewEncoder(mw).Encode(msg)
}

func (f *FileServer) StoreData(key string, r io.Reader) error {
	buf := new(bytes.Buffer)
	tee := io.TeeReader(r, buf)

	size, err := f.store.Write(key, tee)
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{Key: key, Size: size},
	}
	msgBuf := new(bytes.Buffer)
	if err := gob.NewEncoder(msgBuf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range f.peers {
		if err := peer.Send(msgBuf.Bytes()); err != nil {
			log.Println("failed to send key: ", err)
			return err
		}
	}
	time.Sleep(1 * time.Second)
	for _, peer := range f.peers {
		n, err := io.Copy(peer, r)
		if err != nil {
			log.Println("failed to send key: ", err)
			return err
		}
		fmt.Println("received and written", n, "bytes")
	}
	return nil

	// var (
	// 	fileBuffer = new(bytes.Buffer)
	// 	tee        = io.TeeReader(r, fileBuffer)
	// )

	// if err := f.store.Write(key, tee); err != nil {
	// 	log.Println("failed to write payload: ", err)
	// 	return err
	// }

	// p := &DataMessage{
	// 	Key:  key,
	// 	Data: fileBuffer.Bytes(),
	// }
	// return f.broadcast(&Message{
	// 	From:    "todo",
	// 	Payload: p,
	// })
}

func (f *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to user quit action")
		f.Transport.Close()
	}()
	for {
		select {
		case rpc := <-f.Transport.Consume():
			var msg Message

			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("failed to decode payload: ", err, string(msg.Payload.([]byte)))
				continue
			}

			if err := f.handleMessage(rpc.From, &msg); err != nil {
				log.Println("failed to handle message: ", err)
				return
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
	}
	return nil
}

func (fs *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	fmt.Printf("saved file: %v+\n", msg)
	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("peer not found (%s) in peer list", from)
	}

	if _, err := fs.store.Write(msg.Key, io.LimitReader(peer, msg.Size)); err != nil {
		return err
	}
	peer.(*p2p.TCPPeer).Wg.Done()
	return nil
}

func (fs *FileServer) bootstrapNetwork() error {
	for _, addr := range fs.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		log.Println("attempting to connect with address: ", addr)
		go func(addr string) {
			if err := fs.Transport.Dial(addr); err != nil {
				log.Println("dial error: ", err)
			}
		}(addr)
	}
	return nil
}

func (fs *FileServer) Start() error {
	if err := fs.Transport.ListenAndAccept(); err != nil {
		return err
	}

	fs.bootstrapNetwork()

	fs.loop()

	return nil
}

func init() {
	gob.Register(MessageStoreFile{})
}
