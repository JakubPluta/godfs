package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestStore(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)

	key := "mybestpicture"
	data := []byte("some jpg data")

	if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}

	r, err := s.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("written data: %s vs read data: %s\n", data, b)
	if string(b) != string(data) {
		t.Errorf("expected %s, got %s", data, b)
	}

	s.Delete(key)

}

func TestStoreDelete(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)
	key := "mybestpicture"
	if err := s.writeStream(key, bytes.NewReader([]byte("some jpg data"))); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(key); err == nil {
		t.Fatal("expected error")
	}
}

func TestPathTransformFunc(t *testing.T) {
	key := "mybestpicture"
	expectedOriginalKey := "be17b32c2870b1c0c73b59949db6a3be7814dd23"
	expectedPathName := "be17b/32c28/70b1c/0c73b/59949/db6a3/be781/4dd23"
	pathKey := CASPathTransformFunc(key)
	fmt.Println(pathKey)
	if pathKey.Filename != expectedOriginalKey {
		t.Errorf("expected %s, got %s", expectedOriginalKey, pathKey.Filename)
	}
	if pathKey.PathName != expectedPathName {
		t.Errorf("expected %s, got %s", expectedPathName, pathKey.PathName)
	}

}
