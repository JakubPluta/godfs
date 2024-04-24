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
	if _, err := copyDecrypt(keyBuf, dst, out); err != nil {
		t.Fatal(err)
	}
	fmt.Println(out.String())

	if out.String() != txt {
		t.Errorf("expected %s, got %s", txt, out.String())
	}

}
