package gelf

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUDPTransport_SendReceive(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	defer conn.Close()

	tr, err := newUDPTransport(conn.LocalAddr().String())
	require.NoError(t, err)
	defer tr.Close()

	msg := map[string]string{"short_message": "hello"}
	data, _ := json.Marshal(msg)
	require.NoError(t, tr.Send(data))

	buf := make([]byte, 8192)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	require.NoError(t, err)

	var received map[string]string
	require.NoError(t, json.Unmarshal(buf[:n], &received))
	assert.Equal(t, "hello", received["short_message"])
}

func TestUDPTransport_Close(t *testing.T) {
	tr, err := newUDPTransport("127.0.0.1:12201")
	require.NoError(t, err)
	assert.NoError(t, tr.Close())
}
