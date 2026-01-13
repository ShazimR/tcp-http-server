package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderParse(t *testing.T) {
	// Test: Valid headers
	headers := NewHeaders()
	data := []byte("Host: localhost:8080\r\nFooFoo: barbar\r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:8080", headers.Get("Host"))
	assert.Equal(t, "localhost:8080", headers.Get("hOsT"))
	assert.Equal(t, "barbar", headers.Get("FooFoo"))
	assert.Equal(t, "", headers.Get("MissingKey"))
	assert.Equal(t, 40, n)
	assert.True(t, done)

	// Test: Valid headers with OWS
	headers = NewHeaders()
	data = []byte("Host:    localhost:3000     \r\nBarBar: foofoo    \r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:3000", headers.Get("Host"))
	assert.Equal(t, "foofoo", headers.Get("BarBar"))
	assert.Equal(t, "", headers.Get("MissingKey"))
	assert.Equal(t, 52, n)
	assert.True(t, done)

	// Test: Valid headers with multiple values for single headers
	headers = NewHeaders()
	data = []byte("Host: localhost:3000\r\nSet-Person: shazim-rahman\r\nSet-Person: bob-dingo\r\nSet-Person: john-doe\r\nSet-Person: shazim-rahman\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:3000", headers.Get("Host"))
	assert.Equal(t, "shazim-rahman,bob-dingo,john-doe,shazim-rahman", headers.Get("Set-Person"))
	assert.Equal(t, "", headers.Get("MissingKey"))
	assert.Equal(t, 123, n)
	assert.True(t, done)

	// Test: Invalid header spacing
	headers = NewHeaders()
	data = []byte("    Host : localhost:8080       \r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, ErrMalformedFieldLine, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)

	// Test: Invalid header name (uses invalid chars)
	headers = NewHeaders()
	data = []byte("H©st: localhost:8080\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, ErrMalformedHeaderName, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)
}
