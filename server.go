package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"

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
	From    string
	Payload any
}

type DataMessage struct {
	Key  string
	Data []byte
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

	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	if err := f.store.Write(key, tee); err != nil {
		log.Println("failed to write payload: ", err)
		return err
	}

	p := &DataMessage{
		Key:  key,
		Data: fileBuffer.Bytes(),
	}
	return f.broadcast(&Message{
		From:    "todo",
		Payload: p,
	})
}

func (f *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to user quit action")
		f.Transport.Close()
	}()
	for {
		select {
		case msg := <-f.Transport.Consume():
			var m Message

			if err := gob.NewDecoder(bytes.NewReader(msg.Payload)).Decode(&m); err != nil {
				log.Println("failed to decode payload: ", err, string(msg.Payload))
				continue
			}
			if err := f.handleMessage(&m); err != nil {
				log.Println("failed to handle message: ", err)
			}
		case <-f.quitchan:
			return
		}
	}
}

func (fs *FileServer) handleMessage(m *Message) error {
	switch v := m.Payload.(type) {
	case *DataMessage:
		fmt.Printf("saved file: %v+\n", v)
	}
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
