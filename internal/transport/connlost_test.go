package transport

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestIsConnectionLost(t *testing.T) {
	yes := []error{io.EOF, fmt.Errorf("wrap: %w", io.ErrUnexpectedEOF), &ssh.ExitMissingError{}, errors.New("read tcp: connection reset by peer"), errors.New("write: broken pipe")}
	for _, e := range yes {
		if !IsConnectionLost(e) {
			t.Errorf("%v must count as lost connection", e)
		}
	}
	for _, e := range []error{nil, errors.New("permission denied"), errors.New("no such file")} {
		if IsConnectionLost(e) {
			t.Errorf("%v must not count as lost connection", e)
		}
	}
}
