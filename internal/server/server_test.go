package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldClose_HTTP11_DefaultKeepAlive(t *testing.T) {
	req := &request.Request{
		RequestLine: request.RequestLine{
			HttpVersion: "1.1",
		},
		Headers: headers.NewHeaders(),
	}
	w := response.NewWriter(nil)

	close := shouldClose(req, w)
	assert.False(t, close)
}

func TestShouldClose_HTTP11_ConnectionClose(t *testing.T) {
	req := &request.Request{
		RequestLine: request.RequestLine{
			HttpVersion: "1.1",
		},
		Headers: headers.NewHeaders(),
	}
	req.Headers.Set("Connection", "close")

	w := response.NewWriter(nil)

	close := shouldClose(req, w)
	assert.True(t, close)
}

func TestShouldClose_HTTP10_DefaultClose(t *testing.T) {
	req := &request.Request{
		RequestLine: request.RequestLine{
			HttpVersion: "1.0",
		},
		Headers: headers.NewHeaders(),
	}
	w := response.NewWriter(nil)

	close := shouldClose(req, w)
	assert.True(t, close)
}

func TestShouldClose_HTTP10_KeepAlive(t *testing.T) {
	req := &request.Request{
		RequestLine: request.RequestLine{
			HttpVersion: "1.0",
		},
		Headers: headers.NewHeaders(),
	}
	req.Headers.Set("Connection", "keep-alive")

	w := response.NewWriter(nil)

	close := shouldClose(req, w)
	assert.False(t, close)
}

func TestServe_And_Close(t *testing.T) {
	handler := func(w *response.Writer, r *request.Request) error {
		return nil
	}

	s, err := Serve(0, handler, nil)
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Close())
}

func TestServer_HandlesBasicRequest(t *testing.T) {
	handler := func(w *response.Writer, r *request.Request) error {
		body := []byte("ok")
		h := response.GetDefaultHeaders(len(body))
		return w.WriteResponse(response.StatusOK, h, body)
	}

	s, err := Serve(0, handler, nil)
	require.NoError(t, err)
	defer s.Close()

	addr := s.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)

	assert.Contains(t, line, "200")
}

func TestServer_KeepAlive_MultipleRequests(t *testing.T) {
	handler := func(w *response.Writer, r *request.Request) error {
		body := []byte("pong")
		h := headers.NewHeaders()
		h.Set("Content-Length", strconv.Itoa(len(body)))
		return w.WriteResponse(response.StatusOK, h, body)
	}

	s, err := Serve(0, handler, nil)
	require.NoError(t, err)
	defer s.Close()

	addr := s.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	// First request
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", line)
	line, err = reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "content-length: 4\r\n", line)
	line, err = reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "\r\n", line)
	line, err = reader.ReadString('g')
	require.NoError(t, err)
	assert.Equal(t, "pong", line)

	// Second request on same connection
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")

	line, err = reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", line)
	line, err = reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "content-length: 4\r\n", line)
	line, err = reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "\r\n", line)
	line, err = reader.ReadString('g')
	require.NoError(t, err)
	assert.Equal(t, "pong", line)
}

func TestServer_HTTP10_ClosesConnection(t *testing.T) {
	handler := func(w *response.Writer, r *request.Request) error {
		body := []byte("ok")
		h := headers.NewHeaders()
		h.Set("Content-Length", strconv.Itoa(len(body)))
		return w.WriteResponse(response.StatusOK, h, body)
	}

	s, err := Serve(0, handler, nil)
	require.NoError(t, err)
	defer s.Close()

	addr := s.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)

	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")

	reader := bufio.NewReader(conn)
	// status
	_, err = reader.ReadString('\n')
	require.NoError(t, err)
	// header
	_, err = reader.ReadString('\n')
	require.NoError(t, err)
	// body
	_, err = reader.ReadString('k')
	require.NoError(t, err)

	// Give server time to close connection
	time.Sleep(50 * time.Millisecond)

	// Further read should fail
	_, err = reader.ReadString('\n')
	assert.Equal(t, io.EOF, err)
}
