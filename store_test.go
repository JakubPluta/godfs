package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestStore(t *testing.T) {
	s := newStore()
	id := generateID()
	defer teardown(t, s)
	for i := 0; i < 50; i++ {

		key := fmt.Sprintf("foobar_%d", i)
		data := []byte("some jpg data here")

		if _, err := s.writeStream(id, key, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		if ok := s.Has(id, key); !ok {
			t.Fatal("expected true")
		}

		_, r, err := s.Read(id, key)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}

		if string(b) != string(data) {
			t.Errorf("expected %s, got %s", data, b)
		}

		if err := s.Delete(id, key); err != nil {
			t.Fatal(err)
		}
		if ok := s.Has(id, key); ok {
			t.Fatal("expected to not that key exists")
		}
	}

}

func TestStoreDelete(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}

	s := NewStore(opts)
	id := generateID()
	key := "mybestpicture"
	if _, err := s.writeStream(id, key, bytes.NewReader([]byte("some jpg data"))); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(id, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Read(id, key); err == nil {
		t.Fatal("expected error")
	}
}

func TestPathTransformFunc(t *testing.T) {
	key := "mybestpicture"
	expectedOriginalKey := "be17b32c2870b1c0c73b59949db6a3be7814dd23"
	expectedPathName := "be17b/32c28/70b1c/0c73b/59949/db6a3/be781/4dd23"
	pathKey := CASPathTransformFunc(key)
	if pathKey.Filename != expectedOriginalKey {
		t.Errorf("expected %s, got %s", expectedOriginalKey, pathKey.Filename)
	}
	if pathKey.PathName != expectedPathName {
		t.Errorf("expected %s, got %s", expectedPathName, pathKey.PathName)
	}

}

func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
		Root:              "tmp",
	}
	return NewStore(opts)
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
}
