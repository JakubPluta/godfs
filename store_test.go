package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: DefaultPathTransformFunc,
	}
	s := NewStore(opts)
	data := bytes.NewReader([]byte("some jpg data"))
	if err := s.writeStream("myspecialpicture", data); err != nil {
		t.Fatal(err)
	}

}

func TestPathTransformFunc(t *testing.T) {
	key := "mybestpicture"
	pathName := CASPathTransformFunc(key)
	fmt.Println(pathName)
	assert.Equal(t, pathName, "be17b/32c28/70b1c/0c73b/59949/db6a3/be781/4dd23")

}
