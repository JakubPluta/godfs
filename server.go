package main

import (
	"fmt"
	"log"

	"github.com/JakubPluta/godfs/p2p"
)

type FileServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
}

type FileServer struct {
	FileServerOpts
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
	}
}

func (f *FileServer) Stop() {
	close(f.quitchan)
}

func (f *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to user quit action")
		f.Transport.Close()
	}()
	for {
		select {
		case msg := <-f.Transport.Consume():
			fmt.Println("got message: ", msg)
		case <-f.quitchan:
			return
		}
	}
}

func (fs *FileServer) Start() error {
	if err := fs.Transport.ListenAndAccept(); err != nil {
		return err
	}
	fs.loop()

	return nil
}
