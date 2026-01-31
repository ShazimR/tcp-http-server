package response

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
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

type badReadSeeker struct{}

func (badReadSeeker) Read(p []byte) (int, error)                   { return 0, errors.New("readfail") }
func (badReadSeeker) Seek(offset int64, whence int) (int64, error) { return 0, nil }

type badSeek struct {
	data []byte
}

func (b *badSeek) Read(p []byte) (int, error) {
	if len(b.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.data)
	b.data = b.data[n:]
	return n, nil
}

func (b *badSeek) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seekfail")
}

func mkReq(method, target string) *request.Request {
	return &request.Request{
		RequestLine: request.RequestLine{
			Method:        method,
			RequestTarget: target,
		},
		Headers:    headers.NewHeaders(),
		PathParams: make(map[string]string),
		// your Request always has these initialized in RequestFromReader,
		// but for these unit tests we only need headers + requestline.
		RequestParams: make(map[string]string),
	}
}

func statusLineOf(out string) string {
	i := strings.Index(out, "\r\n")
	if i == -1 {
		return out
	}
	return out[:i+2]
}

func headerBlock(out string) string {
	i := strings.Index(out, "\r\n\r\n")
	if i == -1 {
		return out
	}
	return out[:i+4]
}

func bodyOf(out string) string {
	i := strings.Index(out, "\r\n\r\n")
	if i == -1 {
		return ""
	}
	return out[i+4:]
}

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

func TestWritePartialContentResponse_NoRange_Returns200FullBody(t *testing.T) {
	content := []byte("hello-world")
	req := mkReq("GET", "/video")
	// no Range header

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content) // ReadSeeker
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-type: video/mp4\r\n")
	assert.Contains(t, hb, "accept-ranges: bytes\r\n")
	assert.Contains(t, hb, "content-length: 11\r\n")

	assert.Equal(t, "hello-world", bodyOf(out))
}

func TestWritePartialContentResponse_RangeStartOnly_Returns206AndFullToEOF(t *testing.T) {
	content := []byte("0123456789") // 10 bytes
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=0-")

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content)
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 206 Partial Content\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-type: video/mp4\r\n")
	assert.Contains(t, hb, "accept-ranges: bytes\r\n")
	assert.Contains(t, hb, "content-range: bytes 0-9/10\r\n")
	assert.Contains(t, hb, "content-length: 10\r\n")

	assert.Equal(t, "0123456789", bodyOf(out))
}

func TestWritePartialContentResponse_RangeStartEnd_Returns206Subset(t *testing.T) {
	content := []byte("abcdefghijklmnopqrstuvwxyz") // 26 bytes
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=2-5")

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content)
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 206 Partial Content\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-range: bytes 2-5/26\r\n")
	assert.Contains(t, hb, "content-length: 4\r\n")

	assert.Equal(t, "cdef", bodyOf(out))
}

func TestWritePartialContentResponse_RangeEndClamped_Returns206Clamped(t *testing.T) {
	content := []byte("abcdefghij") // 10 bytes
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=7-999")

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content)
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 206 Partial Content\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-range: bytes 7-9/10\r\n")
	assert.Contains(t, hb, "content-length: 3\r\n")

	assert.Equal(t, "hij", bodyOf(out))
}

func TestWritePartialContentResponse_InvalidRange_Returns400(t *testing.T) {
	content := []byte("abcdefghij") // 10 bytes
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=-10")

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content)
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 400 Bad Request\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-type: text/plain\r\n")
	assert.Contains(t, hb, "content-length: 13\r\n")
	assert.Equal(t, "invalid range", bodyOf(out))
}

func TestWritePartialContentResponse_UnsatisfiableStart_Returns416(t *testing.T) {
	content := []byte("abcdefghij") // 10 bytes
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=10-") // start == size => out of bounds

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content)
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 416 Range Not Satisfiable\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-type: text/plain\r\n")
	assert.Contains(t, hb, "content-range: bytes */10\r\n")
	assert.Contains(t, hb, "content-length: 22\r\n") // len("invalid range provided")
	assert.Equal(t, "invalid range provided", bodyOf(out))
}

func TestWritePartialContentResponse_UnsatisfiableEndLtStart_Returns416(t *testing.T) {
	content := []byte("abcdefghij") // 10 bytes
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=7-3")

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := bytes.NewReader(content)
	err := w.WritePartialContentResponse(rs, len(content), "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 416 Range Not Satisfiable\r\n", statusLineOf(out))

	hb := headerBlock(out)
	assert.Contains(t, hb, "content-range: bytes */10\r\n")
	assert.Equal(t, "invalid range provided", bodyOf(out))
}

func TestWritePartialContentResponse_ReadAllError_Returns500(t *testing.T) {
	req := mkReq("GET", "/video")
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WritePartialContentResponse(badReadSeeker{}, 10, "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 500 Internal Server Error\r\n", statusLineOf(out))
	assert.Contains(t, headerBlock(out), "content-type: text/plain\r\n")
	assert.Equal(t, "error loading content", bodyOf(out))
}

func TestWritePartialContentResponse_LoadRangeSeekError_Returns500(t *testing.T) {
	req := mkReq("GET", "/video")
	req.Headers.Set("Range", "bytes=0-")

	var buf bytes.Buffer
	w := NewWriter(&buf)

	rs := &badSeek{data: []byte("abcdefghij")} // 10 bytes
	err := w.WritePartialContentResponse(rs, 10, "video/mp4", req)
	require.NoError(t, err)

	out := buf.String()
	assert.Equal(t, "HTTP/1.1 500 Internal Server Error\r\n", statusLineOf(out))
	assert.Equal(t, "error loading range", bodyOf(out))
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

func TestParseRange(t *testing.T) {
	start, end, endProvided, ok := parseRange("bytes=0-")
	assert.True(t, ok)
	assert.Equal(t, 0, start)
	assert.Equal(t, 0, end)
	assert.False(t, endProvided)

	start, end, endProvided, ok = parseRange("bytes=5-10")
	assert.True(t, ok)
	assert.Equal(t, 5, start)
	assert.Equal(t, 10, end)
	assert.True(t, endProvided)

	_, _, _, ok = parseRange("bytes=-10")
	assert.False(t, ok)

	_, _, _, ok = parseRange("bytes=abc-10")
	assert.False(t, ok)

	_, _, _, ok = parseRange("nope=0-10")
	assert.False(t, ok)

	_, _, _, ok = parseRange("bytes=1") // missing '-'
	assert.False(t, ok)
}

func TestLoadRange(t *testing.T) {
	content := []byte("abcdefghijklmnopqrstuvwxyz") // 26 bytes
	rs := bytes.NewReader(content)

	// bytes=0- (no end provided) => 0..25
	body, usedEnd, err := loadRange(rs, len(content), 0, 0, false)
	require.NoError(t, err)
	assert.Equal(t, 25, usedEnd)
	assert.Equal(t, string(content), string(body))

	// bytes=2-5 => "cdef"
	rs = bytes.NewReader(content)
	body, usedEnd, err = loadRange(rs, len(content), 2, 5, true)
	require.NoError(t, err)
	assert.Equal(t, 5, usedEnd)
	assert.Equal(t, "cdef", string(body))

	// clamp end: bytes=20-999 => 20..25
	rs = bytes.NewReader(content)
	body, usedEnd, err = loadRange(rs, len(content), 20, 999, true)
	require.NoError(t, err)
	assert.Equal(t, 25, usedEnd)
	assert.Equal(t, "uvwxyz", string(body))

	// start out of bounds
	rs = bytes.NewReader(content)
	_, _, err = loadRange(rs, len(content), 26, 0, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRangeOutOfBounds)

	// end < start
	rs = bytes.NewReader(content)
	_, _, err = loadRange(rs, len(content), 10, 5, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRangeEndLtStart)

	// contentSize <= 0
	rs = bytes.NewReader(content)
	_, _, err = loadRange(rs, 0, 0, 0, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRangeOutOfBounds)
}
