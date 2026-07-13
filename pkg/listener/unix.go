package listener

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// NewUnixListener creates an HTTP listener on a Unix domain socket. A stale
// socket file is removed, but an active socket or regular file is never replaced.
func NewUnixListener(path string) (net.Listener, error) {
	if path == "" {
		return nil, fmt.Errorf("Unix socket path is empty")
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("Unix socket path %q exists and is not a socket", path)
		}
		conn, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("Unix socket %q is already in use", path)
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("check existing Unix socket %q: %w", path, dialErr)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale Unix socket %q: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect Unix socket path %q: %w", path, err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on Unix socket %q: %w", path, err)
	}
	return &unixListener{Listener: ln, path: path}, nil
}

type unixListener struct {
	net.Listener
	path string
}

func (l *unixListener) Close() error {
	err := l.Listener.Close()
	removeErr := os.Remove(l.path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	if err != nil {
		return err
	}
	return removeErr
}
