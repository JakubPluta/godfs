package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestCopyEncryptDecrypt(t *testing.T) {

	txt := "Foo not Bar"

	src := bytes.NewReader([]byte(txt))
	dst := new(bytes.Buffer)
	keyBuf := newEncryptionKey()
	_, err := copyEncrypt(keyBuf, src, dst)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(dst.String())

	out := new(bytes.Buffer)
	nw, err := copyDecrypt(keyBuf, dst, out)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(out.String())

	if nw != len(txt)+16 {
		t.Errorf("expected %d, got %d", len(txt)+16, nw)

	}

	if out.String() != txt {
		t.Errorf("expected %s, got %s", txt, out.String())
	}

}
