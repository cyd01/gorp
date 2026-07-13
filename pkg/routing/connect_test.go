package routing

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/cyd01/gorp/pkg/backend"
	"github.com/cyd01/gorp/pkg/selector"
)

func TestRouteConnect(t *testing.T) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendListener.Close()
	go func() {
		conn, err := backendListener.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(conn, conn)
		}
	}()

	backendURL, _ := url.Parse("tcp://" + backendListener.Addr().String())
	be := &backend.Backend{Name: "echo", URL: backendURL, ConnectTimeout: time.Second}
	be.Healthy.Store(true)
	route := &Route{
		Prefix:   "/",
		Selector: selector.NewRandomSelector([]*backend.Backend{be}),
	}

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: New([]*Route{route}, "")}
	go server.Serve(proxyListener)
	defer server.Close()

	client, err := net.Dial("tcp", proxyListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, _ = fmt.Fprintf(client, "CONNECT target.example:443 HTTP/1.1\r\nHost: target.example:443\r\n\r\n")

	reader := bufio.NewReader(client)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if response != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("response = %q", response)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}

	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("tunnel response = %q", got)
	}
}
