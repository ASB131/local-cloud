package files

import (
	"errors"
	"io"
	"syscall"
	"testing"
)

func TestIsStreamClientAbort(t *testing.T) {
	if IsStreamClientAbort(syscall.EPIPE) != true {
		t.Fatal("EPIPE")
	}
	if IsStreamClientAbort(errors.New("write: broken pipe")) != true {
		t.Fatal("broken pipe message")
	}
	if IsStreamClientAbort(io.EOF) != true {
		t.Fatal("EOF")
	}
	if IsStreamClientAbort(errors.New("disk full")) != false {
		t.Fatal("unexpected abort")
	}
}
