package response

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chunkWriter struct {
	maxPerWrite int
	buf         bytes.Buffer
}

// Simulates writing a variable number of bytes per chunk to a network.
func (cw *chunkWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if cw.maxPerWrite <= 0 || len(p) <= cw.maxPerWrite {
		_, _ = cw.buf.Write(p)
		return len(p), nil
	}
	_, _ = cw.buf.Write(p[:cw.maxPerWrite])
	return cw.maxPerWrite, nil
}

func (cw *chunkWriter) String() string { return cw.buf.String() }

type errWriter struct {
	failAfter int
	writes    int
}

func (ew *errWriter) Write(p []byte) (int, error) {
	ew.writes++
	if ew.writes > ew.failAfter {
		return 0, errors.New("boom")
	}
	return len(p), nil
}

type zeroWriter struct{}

func (zw *zeroWriter) Write(p []byte) (int, error) { return 0, nil }

func TestWriteStatusLine(t *testing.T) {
	// Test: Good status line
	cw := &chunkWriter{maxPerWrite: 3}
	w := NewWriter(cw)
	err := w.WriteStatusLine(StatusOK)
	require.NoError(t, err)
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", cw.String())

	// Test: Unrecognized status code
	cw = &chunkWriter{maxPerWrite: 64}
	w = NewWriter(cw)
	err = w.WriteStatusLine(StatusCode(999))
	require.Error(t, err)
	assert.Equal(t, ErrUnrecognizedStatusCode, err)

	// Test: Underlying writer error
	ew := &errWriter{failAfter: 0}
	w = NewWriter(ew)
	err = w.WriteStatusLine(StatusOK)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Zero-byte writes
	zw := &zeroWriter{}
	w = NewWriter(zw)
	err = w.WriteStatusLine(StatusOK)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))
}

func TestWriteHeaders(t *testing.T) {
	// Test: Valid headers
	cw := &chunkWriter{maxPerWrite: 2}
	w := NewWriter(cw)
	h := headers.NewHeaders()
	h.Set("Host", "localhost:8080")
	h.Set("Content-Type", "text/plain")
	h.Set("X-Test", "abc")
	err := w.WriteHeaders(h)
	require.NoError(t, err)
	out := cw.String()
	assert.True(t, strings.HasSuffix(out, "\r\n\r\n"))
	assert.Contains(t, out, "host: localhost:8080\r\n")
	assert.Contains(t, out, "content-type: text/plain\r\n")
	assert.Contains(t, out, "x-test: abc\r\n")

	// Test: Underlying writer error
	ew := &errWriter{failAfter: 0}
	w = NewWriter(ew)
	err = w.WriteHeaders(h)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Zero-byte writes
	zw := &zeroWriter{}
	w = NewWriter(zw)
	err = w.WriteHeaders(h)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))
}

func TestWriteBody(t *testing.T) {
	// Test: Writes full body under partial writes
	cw := &chunkWriter{maxPerWrite: 1}
	w := NewWriter(cw)
	body := []byte("Hello World!\n")
	err := w.WriteBody(body)
	require.NoError(t, err)
	assert.Equal(t, string(body), cw.String())

	// Test: Underlying writer error
	ew := &errWriter{failAfter: 0}
	w = NewWriter(ew)
	err = w.WriteBody([]byte("hi"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Zero-byte writes
	zw := &zeroWriter{}
	w = NewWriter(zw)
	err = w.WriteBody([]byte("hi"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))
}

func TestWriteChunk(t *testing.T) {
	// Test: Writes chunk framing and data (supports partial writes)
	cw := &chunkWriter{maxPerWrite: 2}
	w := NewWriter(cw)

	payload := []byte("hello") // len=5 -> "5\r\nhello\r\n"
	err := w.WriteChunk(payload)
	require.NoError(t, err)
	assert.Equal(t, "5\r\nhello\r\n", cw.String())

	// Test: Underlying writer error
	ew := &errWriter{failAfter: 0} // fail on first write
	w = NewWriter(ew)
	err = w.WriteChunk([]byte("x"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Zero-byte writes are treated as failure
	zw := &zeroWriter{}
	w = NewWriter(zw)
	err = w.WriteChunk([]byte("x"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))
}

func TestWriteChunkEnd(t *testing.T) {
	// Test: hasTrailers=true writes only "0\r\n"
	cw := &chunkWriter{maxPerWrite: 1}
	w := NewWriter(cw)

	err := w.WriteChunkEnd(true)
	require.NoError(t, err)
	assert.Equal(t, "0\r\n", cw.String())

	// Test: hasTrailers=false writes final CRLF "0\r\n\r\n"
	cw = &chunkWriter{maxPerWrite: 1}
	w = NewWriter(cw)

	err = w.WriteChunkEnd(false)
	require.NoError(t, err)
	assert.Equal(t, "0\r\n\r\n", cw.String())

	// Test: underlying writer error
	ew := &errWriter{failAfter: 0} // fail on first write
	w = NewWriter(ew)

	err = w.WriteChunkEnd(true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: zero-byte writes are treated as failure
	zw := &zeroWriter{}
	w = NewWriter(zw)

	err = w.WriteChunkEnd(false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))
}

func TestWriteResponse(t *testing.T) {
	// Test: Writes status line + headers + body in correct order (supports partial writes)
	cw := &chunkWriter{maxPerWrite: 3}
	w := NewWriter(cw)
	body := []byte("OK")
	h := headers.NewHeaders()
	h.Set("Content-Length", "2")
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/plain")
	err := w.WriteResponse(StatusOK, h, body)
	require.NoError(t, err)
	out := cw.String()

	// starts with status line
	require.True(t, strings.HasPrefix(out, "HTTP/1.1 200 OK\r\n"))

	// header terminator exists
	sep := "\r\n\r\n"
	idx := strings.Index(out, sep)
	require.NotEqual(t, -1, idx)

	// body appears after the blank line
	assert.Equal(t, "OK", out[idx+len(sep):])

	// and headers are somewhere before the terminator (order-independent)
	headerSection := out[:idx+len(sep)]
	assert.Contains(t, headerSection, "content-length: 2\r\n")
	assert.Contains(t, headerSection, "connection: close\r\n")
	assert.Contains(t, headerSection, "content-type: text/plain\r\n")

	// Test: Fails fast if status line write fails
	ew := &errWriter{failAfter: 0} // fail on first Write
	w = NewWriter(ew)
	err = w.WriteResponse(StatusOK, h, body)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Fails if headers write fails (status line succeeds, headers fail)
	ew = &errWriter{failAfter: 1} // 1st write ok (status line), 2nd write fails (headers)
	w = NewWriter(ew)
	err = w.WriteResponse(StatusOK, h, body)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Fails if headers write fails (status line succeeds, headers fail)
	ew = &errWriter{failAfter: 2} // 1st 2 writes ok (status line, headers), 3rd write fails (body)
	w = NewWriter(ew)
	err = w.WriteResponse(StatusOK, h, body)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))

	// Test: Zero-byte writes are treated as failure
	zw := &zeroWriter{}
	w = NewWriter(zw)
	err = w.WriteResponse(StatusOK, h, body)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFailedToWrite))
}

func TestGetDefaultHeaders(t *testing.T) {
	h := GetDefaultHeaders(123)
	v, ok := h.Get("Content-Length")
	require.True(t, ok)
	assert.Equal(t, "123", v)
	v, ok = h.Get("Connection")
	require.True(t, ok)
	assert.Equal(t, "close", v)
	v, ok = h.Get("Content-Type")
	require.True(t, ok)
	assert.Equal(t, "text/html", v)
}
