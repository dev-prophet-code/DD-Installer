package transport

import (
	"errors"
	"io"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
)

// IsConnectionLost reports whether err means the transport itself broke (SSH
// connection dropped, network gone) as opposed to a command that simply
// failed. Such errors are retried after reconnecting.
func IsConnectionLost(err error) bool {
	if err == nil {
		return false
	}
	var missing *ssh.ExitMissingError
	if errors.As(err, &missing) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, p := range []string{"connection reset", "broken pipe", "use of closed network connection",
		"connection lost", "ssh: disconnect", "i/o timeout", "connection timed out", "eof"} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
