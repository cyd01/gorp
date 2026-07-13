package listener

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestSOCKS5ConnectTunnel(t *testing.T) {
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoListener.Close()
	go func() {
		connection, acceptErr := echoListener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			_, _ = io.Copy(connection, connection)
		}
	}()

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer socksListener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = HandleSOCKS5(ctx, socksListener, time.Second, false) }()

	client, err := net.Dial("tcp", socksListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte{socks5Version, 1, socks5NoAuth}); err != nil {
		t.Fatal(err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(client, methodReply); err != nil {
		t.Fatal(err)
	}
	if methodReply[0] != socks5Version || methodReply[1] != socks5NoAuth {
		t.Fatalf("method reply = %v", methodReply)
	}

	target := echoListener.Addr().(*net.TCPAddr)
	request := []byte{socks5Version, socks5Connect, 0, socks5IPv4}
	request = append(request, target.IP.To4()...)
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, uint16(target.Port))
	request = append(request, port...)
	if _, err := client.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatal(err)
	}
	if reply[0] != socks5Version || reply[1] != socks5Succeeded {
		t.Fatalf("connect reply = %v", reply)
	}

	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(client, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("tunnel response = %q", got)
	}
}

func TestSOCKS5UsernamePasswordAuthentication(t *testing.T) {
	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer socksListener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = HandleSOCKS5(ctx, socksListener, time.Second, false, map[string]string{"alice": "secret"}) }()

	client, err := net.Dial("tcp", socksListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte{socks5Version, 2, socks5NoAuth, socks5UsernamePass}); err != nil {
		t.Fatal(err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(client, methodReply); err != nil {
		t.Fatal(err)
	}
	if methodReply[1] != socks5UsernamePass {
		t.Fatalf("method reply = %v", methodReply)
	}
	if _, err := client.Write([]byte{socks5AuthVersion, 5, 'a', 'l', 'i', 'c', 'e', 6, 's', 'e', 'c', 'r', 'e', 't'}); err != nil {
		t.Fatal(err)
	}
	authReply := make([]byte, 2)
	if _, err := io.ReadFull(client, authReply); err != nil {
		t.Fatal(err)
	}
	if authReply[1] != 0 {
		t.Fatalf("authentication reply = %v", authReply)
	}
}
