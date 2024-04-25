package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const defaultRootFolderName = "godfs"

type PathTransformFunc func(string) PathKey
type PathKey struct {
	PathName string
	Filename string
}

func (p *PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", p.PathName, p.Filename)
}
func (p *PathKey) FirstPathName() string {
	paths := strings.Split(p.PathName, "/")
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// Content Addressable Storage Transform Function
// implements type of CAS (Content Addressable Storage) and it's a type of PathTransformFunc
func CASPathTransformFunc(key string) PathKey {
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
	return PathKey{
		PathName: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

type StoreOpts struct {
	// Root is the folder name of the root, containing all files in the store
	Root              string
	PathTransformFunc PathTransformFunc
}

var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		PathName: key,
		Filename: key,
	}
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if len(opts.Root) == 0 {
		// default
		opts.Root = defaultRootFolderName
	}
	return &Store{
		StoreOpts: opts,
	}
}

func (s *Store) Read(id, key string) (int64, io.Reader, error) {
	return s.readStream(id, key)
}

func (s *Store) readStream(id, key string) (int64, io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	file, err := os.Open(fullPathWithRoot)
	if err != nil {
		return 0, nil, err
	}

	fi, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}

	return fi.Size(), file, nil
}

func (s *Store) Has(id string, key string) bool {
	pathKey := s.PathTransformFunc(key)
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	_, err := os.Stat(fullPathWithRoot)
	return !errors.Is(err, os.ErrNotExist)
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

func (s *Store) Delete(id, key string) error {
	pathKey := s.PathTransformFunc(key)
	firstPathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FirstPathName())
	defer func() {
		log.Printf("deleted %s\n", pathKey.Filename)
	}()
	return os.RemoveAll(firstPathNameWithRoot)
}

func (s *Store) Write(id, key string, r io.Reader) (int64, error) {
	return s.writeStream(id, key, r)
}

func (s *Store) WriteDecrypt(encKey []byte, id, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWrite(id, key)
	if err != nil {
		return 0, err
	}
	n, err := copyDecrypt(encKey, r, f)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func (s *Store) openFileForWrite(id string, key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)
	pathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.PathName)
	if err := os.MkdirAll(pathNameWithRoot, os.ModePerm); err != nil {
		return nil, err
	}

	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	return os.Create(fullPathWithRoot)
}

func (s *Store) writeStream(id, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWrite(id, key)
	if err != nil {
		return 0, err
	}
	return io.Copy(f, r)

}
