package gelf

import (
	"fmt"
	"net"
)

type udpTransport struct{ conn *net.UDPConn }

func newUDPTransport(address string) (*udpTransport, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolve udp: %w", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}
	return &udpTransport{conn: conn}, nil
}

func (t *udpTransport) Send(data []byte) error { _, err := t.conn.Write(data); return err }
func (t *udpTransport) Close() error            { return t.conn.Close() }
