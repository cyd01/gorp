package listener

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyd01/gorp/pkg/config"
)

func TestUnixListenerServesHTTPAndRemovesSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http.sock")
	ln, err := NewUnixListener(path)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}
	go func() { _ = server.Serve(ln) }()

	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	client := &http.Client{Transport: transport}
	response, err := client.Get("http://unix/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket path still exists after shutdown, lstat error = %v", err)
	}
}

func TestNewUnixListenerRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(path, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewUnixListener(path); err == nil {
		t.Fatal("NewUnixListener() accepted an existing regular file")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("existing regular file was removed: %v", err)
	}
}

func TestNewUnixListenerRemovesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.sock")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	ln, err := NewUnixListener(path)
	if err != nil {
		t.Fatalf("NewUnixListener() failed to replace stale socket: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUnixTCPListenerForwardsStream(t *testing.T) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendListener.Close()
	go func() {
		connection, acceptErr := backendListener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			_, _ = io.Copy(connection, connection)
		}
	}()

	backendURL, err := url.Parse("tcp://" + backendListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tcp.sock")
	ln, err := NewUnixTCPListener(path, "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = HandleTCPListener(ctx, ln, []config.Backend{{Name: "echo", URL: backendURL.String()}})
	}()

	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write([]byte("unix tcp")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len("unix tcp"))
	if _, err := io.ReadFull(client, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "unix tcp" {
		t.Fatalf("received %q, want %q", got, "unix tcp")
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Unix TCP socket still exists after close, lstat error = %v", err)
	}
}
