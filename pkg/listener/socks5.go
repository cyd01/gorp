package listener

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const (
	socks5Version       = 0x05
	socks5NoAuth        = 0x00
	socks5UsernamePass  = 0x02
	socks5NoAcceptable  = 0xff
	socks5Connect       = 0x01
	socks5IPv4          = 0x01
	socks5Domain        = 0x03
	socks5IPv6          = 0x04
	socks5Succeeded     = 0x00
	socks5GeneralError  = 0x01
	socks5CommandDenied = 0x07
	socks5AuthVersion   = 0x01
)

func HandleSOCKS5(ctx context.Context, listener net.Listener, timeout time.Duration, logflag bool, configuredUsers ...map[string]string) error {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	for {
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			_ = tcpListener.SetDeadline(time.Now().Add(time.Second))
		}
		client, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					continue
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
		go handleSOCKS5Connection(client, timeout, logflag, firstUsers(configuredUsers))
	}
}

func handleSOCKS5Connection(client net.Conn, timeout time.Duration, logflag bool, users map[string]string) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(timeout))
	if err := negotiateSOCKS5(client, users); err != nil {
		return
	}
	target, err := readSOCKS5ConnectRequest(client)
	if err != nil {
		_ = writeSOCKS5Reply(client, socks5GeneralError, nil)
		return
	}
	if logflag {
		log.Printf("[SOCKS5] %s %s", target, client.RemoteAddr().String())
	}
	upstream, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		_ = writeSOCKS5Reply(client, socks5GeneralError, nil)
		return
	}
	defer upstream.Close()
	_ = client.SetDeadline(time.Time{})
	if err := writeSOCKS5Reply(client, socks5Succeeded, upstream.LocalAddr()); err != nil {
		return
	}

	clientToUpstream := make(chan struct{})
	go func() {
		_, _ = io.Copy(upstream, client)
		closeWrite(upstream)
		close(clientToUpstream)
	}()
	_, _ = io.Copy(client, upstream)
	closeWrite(client)
	<-clientToUpstream
}

func negotiateSOCKS5(conn net.Conn, users map[string]string) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != socks5Version {
		return fmt.Errorf("unsupported SOCKS version %d", header[0])
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	if len(users) == 0 {
		for _, method := range methods {
			if method == socks5NoAuth {
				_, err := conn.Write([]byte{socks5Version, socks5NoAuth})
				return err
			}
		}
	} else {
		for _, method := range methods {
			if method == socks5UsernamePass {
				if _, err := conn.Write([]byte{socks5Version, socks5UsernamePass}); err != nil {
					return err
				}
				return authenticateSOCKS5UserPass(conn, users)
			}
		}
	}
	_, _ = conn.Write([]byte{socks5Version, socks5NoAcceptable})
	return fmt.Errorf("SOCKS5 client does not offer a supported authentication method")
}

func authenticateSOCKS5UserPass(conn net.Conn, users map[string]string) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != socks5AuthVersion {
		return fmt.Errorf("unsupported SOCKS5 authentication version %d", header[0])
	}
	username := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, username); err != nil {
		return err
	}
	passwordLength := []byte{0}
	if _, err := io.ReadFull(conn, passwordLength); err != nil {
		return err
	}
	password := make([]byte, int(passwordLength[0]))
	if _, err := io.ReadFull(conn, password); err != nil {
		return err
	}
	status := byte(0xff)
	if expected, ok := users[string(username)]; ok && expected == string(password) {
		status = 0x00
	}
	if _, err := conn.Write([]byte{socks5AuthVersion, status}); err != nil {
		return err
	}
	if status != 0x00 {
		return fmt.Errorf("invalid SOCKS5 username or password")
	}
	return nil
}

func readSOCKS5ConnectRequest(conn net.Conn) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	if header[0] != socks5Version || header[1] != socks5Connect {
		_ = writeSOCKS5Reply(conn, socks5CommandDenied, nil)
		return "", fmt.Errorf("unsupported SOCKS5 request")
	}

	var host string
	switch header[3] {
	case socks5IPv4:
		address := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", err
		}
		host = net.IP(address).String()
	case socks5Domain:
		length := []byte{0}
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", err
		}
		address := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", err
		}
		host = string(address)
	case socks5IPv6:
		address := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", err
		}
		host = net.IP(address).String()
	default:
		return "", fmt.Errorf("unsupported SOCKS5 address type %d", header[3])
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", binary.BigEndian.Uint16(portBytes))), nil
}

func writeSOCKS5Reply(conn net.Conn, status byte, address net.Addr) error {
	bindHost := net.IPv4zero
	bindPort := 0
	if tcpAddress, ok := address.(*net.TCPAddr); ok {
		bindHost = tcpAddress.IP
		bindPort = tcpAddress.Port
	}
	if bindHost == nil || bindHost.To4() != nil {
		bindHost = bindHost.To4()
		if bindHost == nil {
			bindHost = net.IPv4zero
		}
		return writeSOCKS5Bytes(conn, status, socks5IPv4, bindHost, bindPort)
	}
	return writeSOCKS5Bytes(conn, status, socks5IPv6, bindHost, bindPort)
}

func writeSOCKS5Bytes(conn net.Conn, status, addressType byte, address net.IP, port int) error {
	response := []byte{socks5Version, status, 0x00, addressType}
	if addressType == socks5IPv4 {
		response = append(response, address.To4()...)
	} else {
		response = append(response, address.To16()...)
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	response = append(response, portBytes...)
	_, err := conn.Write(response)
	return err
}

func StartSOCKS5(address string, logflag bool) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Printf("SOCKS5 listener started on %s\n", address)
	return HandleSOCKS5(context.Background(), listener, 30*time.Second, logflag)
}
