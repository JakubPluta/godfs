package main

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	"os"
	"strings"
)

type PathTransformFunc func(string) string

// Content Addressable Storage Transform Function
// implements type of CAS (Content Addressable Storage) and it's a type of PathTransformFunc
func CASPathTransformFunc(key string) string {
	// hash the key with sha1
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:])
	blockSize := 5
	sliceLen := len(hashStr) / blockSize
	paths := make([]string, sliceLen)
	for i := 0; i < sliceLen; i++ {
		// 5 byte block
		paths[i] = hashStr[i*blockSize : (i+1)*blockSize]
	}
	return strings.Join(paths, "/")

}

type StoreOpts struct {
	PathTransformFunc PathTransformFunc
}

var DefaultPathTransformFunc = func(key string) string {
	return key
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	return &Store{
		StoreOpts: opts,
	}
}

func (s *Store) writeStream(key string, r io.Reader) error {
	pathName := s.PathTransformFunc(key)
	if err := os.MkdirAll(pathName, os.ModePerm); err != nil {
		return err
	}
	fileName := "somefilename"
	pathAndFileName := pathName + "/" + fileName

	f, err := os.Create(pathAndFileName)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	log.Printf("written %d bytes to disk: %s\n", n, pathAndFileName)
	return nil

}
