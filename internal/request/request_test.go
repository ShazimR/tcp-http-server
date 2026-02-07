package request

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chunkReader struct {
	data            string
	numBytesPerRead int
	pos             int
}

// Simulates reading a variable number of bytes per chunk from a network
func (cr *chunkReader) Read(p []byte) (n int, err error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}
	endIndex := min(cr.pos+cr.numBytesPerRead, len(cr.data))
	n = copy(p, cr.data[cr.pos:endIndex])
	cr.pos += n
	if n > cr.numBytesPerRead {
		n = cr.numBytesPerRead
		cr.pos -= n - cr.numBytesPerRead
	}
	return n, nil
}

func TestRequestLineParse(t *testing.T) {
	// Test: Good GET Request line
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/", r.RequestLine.RequestTarget)
	assert.Equal(t, "1.1", r.RequestLine.HttpVersion)
	assert.Equal(t, 0, len(r.RequestParams))

	// Test: Good GET Request line with path
	reader = &chunkReader{
		data:            "GET /coffee HTTP/1.1\r\nHost: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/coffee", r.RequestLine.RequestTarget)
	assert.Equal(t, "1.1", r.RequestLine.HttpVersion)
	assert.Equal(t, 0, len(r.RequestParams))

	// Test: Good GET Request line with query parameters
	reader = &chunkReader{
		data:            "GET /coffee?size=medium&type=black&name=shazim&test=a=b HTTP/1.1\r\nHost: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/coffee", r.RequestLine.RequestTarget)
	assert.Equal(t, "1.1", r.RequestLine.HttpVersion)
	assert.Equal(t, 4, len(r.RequestParams))
	assert.Equal(t, "medium", r.RequestParams["size"])
	assert.Equal(t, "black", r.RequestParams["type"])
	assert.Equal(t, "shazim", r.RequestParams["name"])
	assert.Equal(t, "a=b", r.RequestParams["test"])

	// Test: Invalid number of parts in the request line
	testReq := "/coffee HTTP/1.1\r\nHost: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n"
	reader = &chunkReader{
		data:            testReq,
		numBytesPerRead: len(testReq),
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
	require.Nil(t, r)
	assert.Equal(t, ErrMalformedRequestLine, err)

	// Test: Invalid HTTP version
	reader = &chunkReader{
		data:            "GET / HTTP/2.0\r\nHost: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
	require.Nil(t, r)
	assert.Equal(t, ErrUnsupportedVersion, err)
}

func TestParseHeaders(t *testing.T) {
	// Test: Standard headers
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 1,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	hostStr, ok := r.Headers.Get("Host")
	assert.True(t, ok)
	assert.Equal(t, "localhost:8080", hostStr)
	userStr, ok := r.Headers.Get("User-Agent")
	assert.True(t, ok)
	assert.Equal(t, "curl/7.81.0", userStr)
	acceptStr, ok := r.Headers.Get("Accept")
	assert.True(t, ok)
	assert.Equal(t, "*/*", acceptStr)

	// Test: Malformed header
	reader = &chunkReader{
		data:            "GET / HTTP/1.1\r\nH©st: localhost:8080\r\nUser-Agent: curl/7.81.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
}

func TestParseBody(t *testing.T) {
	// Test: Standard body
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Content-Length: 13\r\n" +
			"\r\n" +
			"Hello World!\n",
		numBytesPerRead: 1,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Hello World!\n", string(r.Body))

	// Test: Chunk encoded body
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			" W\r\n" +
			"4\r\n" +
			"orld\r\n" +
			"2\r\n" +
			"!\n\r\n" +
			"0\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Hello World!\n", string(r.Body))

	// Test: Chunk encoded body with trailer
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"Trailer: Expires\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			" W\r\n" +
			"4\r\n" +
			"orld\r\n" +
			"2\r\n" +
			"!\n\r\n" +
			"0\r\n" +
			"Expires: tomorrow\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Hello World!\n", string(r.Body))

	// Test: Chunk encoded with CRLF in body
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			"\r\n\r\n" + // crlf in body here
			"1\r\n" +
			"W\r\n" +
			"4\r\n" +
			"orld\r\n" +
			"3\r\n" +
			"!\r\n\r\n" + // crlf in body here
			"0\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Hello\r\nWorld!\r\n", string(r.Body))

	// Test: Chunk encoded with missing CRLF at end of chunk
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			" W\r\n" +
			"4\r\n" +
			"orld" + // missing crlf here
			"3\r\n" +
			"!\r\n" +
			"0\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
	assert.Equal(t, ErrMalformedChunkedBody, err)

	// Test: Chunk encoded chunk size not valid
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			" W\r\n" +
			"4" + // missing crlf causes invalid size -> "4orld\r\n"
			"orld\r\n" +
			"3\r\n" +
			"!\r\n" +
			"0\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
	assert.Equal(t, ErrMalformedChunkedBody, err)

	// Test: Body shorter than reported content length
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Content-Length: 20\r\n" +
			"\r\n" +
			"partial content\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
	assert.Equal(t, io.EOF, err)
}

func TestParseTrailer(t *testing.T) {
	// Test: Trailer header included
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"Trailer: Expires,Another\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			" W\r\n" +
			"4\r\n" +
			"orld\r\n" +
			"2\r\n" +
			"!\n\r\n" +
			"0\r\n" +
			"Expires: tomorrow\r\n" +
			"Another: value\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	expStr, ok := r.Trailer.Get("Expires")
	assert.True(t, ok)
	assert.Equal(t, "tomorrow", expStr)
	anoStr, ok := r.Trailer.Get("Another")
	assert.True(t, ok)
	assert.Equal(t, "value", anoStr)
	dneStr, ok := r.Trailer.Get("DoesNotExist")
	assert.False(t, ok)
	assert.Equal(t, "", dneStr)

	// Test: Trailer header not included
	reader = &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:8080\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"3\r\n" +
			"Hel\r\n" +
			"2\r\n" +
			"lo\r\n" +
			"2\r\n" +
			" W\r\n" +
			"4\r\n" +
			"orld\r\n" +
			"2\r\n" +
			"!\n\r\n" +
			"0\r\n" +
			"Expires: tomorrow\r\n" +
			"Another: value\r\n" +
			"\r\n",
		numBytesPerRead: 1,
	}
	r, err = RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	expStr, ok = r.Trailer.Get("Expires")
	assert.False(t, ok)
	assert.Equal(t, "", expStr)
	anoStr, ok = r.Trailer.Get("Another")
	assert.False(t, ok)
	assert.Equal(t, "", anoStr)
}
