package router

import (
	"bytes"
	"testing"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouterRegistrationDoesNotPanic(t *testing.T) {
	// Test: registering a route should not panic
	r := NewRouter()

	require.NotPanics(t, func() {
		r.GET("/", func(w *response.Writer, req *request.Request) error { return nil })
	})
}

func TestRouterGetHandlerExactMatch(t *testing.T) {
	r := NewRouter()

	hRoot := func(w *response.Writer, req *request.Request) error {
		body := []byte("root")
		h := response.GetDefaultHeaders(len(body))
		return w.WriteResponse(response.StatusOK, h, body)
	}
	hCoffeeGet := func(w *response.Writer, req *request.Request) error {
		body := []byte("coffee-get")
		h := response.GetDefaultHeaders(len(body))
		return w.WriteResponse(response.StatusOK, h, body)
	}
	hCoffeePost := func(w *response.Writer, req *request.Request) error {
		body := []byte("coffee-post")
		h := response.GetDefaultHeaders(len(body))
		return w.WriteResponse(response.StatusOK, h, body)
	}

	r.GET("/", hRoot)
	r.GET("/coffee", hCoffeeGet)
	r.POST("/coffee", hCoffeePost)

	// Test: GET /
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "\r\n\r\nroot")
	}

	// Test: GET /coffee
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/coffee",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/coffee",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "\r\n\r\ncoffee-get")
	}

	// Test: POST /coffee
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "POST",
				RequestTarget: "/coffee",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "POST",
				RequestTarget: "/coffee",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "\r\n\r\ncoffee-post")
	}
}

func TestRouterGetHandlerQueryParam(t *testing.T) {
	r := NewRouter()

	hNoParams := func(w *response.Writer, req *request.Request) error {
		body := []byte("no params")
		h := response.GetDefaultHeaders(len(body))
		return w.WriteResponse(response.StatusOK, h, body)
	}
	hParams := func(w *response.Writer, req *request.Request) error {
		body := []byte("a and b")
		h := response.GetDefaultHeaders(len(body))
		return w.WriteResponse(response.StatusOK, h, body)
	}

	r.GET("/", hNoParams)
	r.GET("/?&a&b", hParams)

	// Test: GET /
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "\r\n\r\nno params")
	}

	// Test: GET /?a=true&b=false
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/?a=true&b=false",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/?a=true&b=false",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "\r\n\r\na and b")
	}

	// Test: GET /?b=false&a=true
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/?b=false&a=true",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/?b=false&a=true",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "\r\n\r\na and b")
	}

	// Test: GET /?a=true
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/?a=true",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/?a=true",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "HTTP/1.1 404 Not Found\r\n")
	}

	// Test: POST /?a=true&b=false
	{
		got := r.GetHandler(&request.Request{
			RequestLine: request.RequestLine{
				Method:        "POST",
				RequestTarget: "/?a=true&b=false",
			},
		})
		require.NotNil(t, got)

		var buf bytes.Buffer
		w := response.NewWriter(&buf)
		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "POST",
				RequestTarget: "/?a=true&b=false",
			},
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "HTTP/1.1 405 Method Not Allowed\r\n")
	}
}

func TestRouterNotFoundVsMethodNotAllowed(t *testing.T) {
	r := NewRouter()

	onlyGet := func(w *response.Writer, req *request.Request) error { return nil }
	r.GET("/only-get", onlyGet)

	// Test: path exists but wrong method => 405 handler
	got := r.GetHandler(&request.Request{
		RequestLine: request.RequestLine{
			Method:        "POST",
			RequestTarget: "/only-get",
		},
	})
	require.NotNil(t, got)

	{
		var buf bytes.Buffer
		w := response.NewWriter(&buf)

		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "POST",
				RequestTarget: "/only-get",
			},
		})
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "HTTP/1.1 405 Method Not Allowed\r\n")
	}

	// Test: path does not exist anywhere => 404 handler
	got = r.GetHandler(&request.Request{
		RequestLine: request.RequestLine{
			Method:        "GET",
			RequestTarget: "/does-not-exist",
		},
	})
	require.NotNil(t, got)

	{
		var buf bytes.Buffer
		w := response.NewWriter(&buf)

		err := got(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/does-not-exist",
			},
		})
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "HTTP/1.1 404 Not Found\r\n")
	}
}

func TestDefaultHandlersWriteEmptyBody(t *testing.T) {
	// Test: notFoundHandler writes Content-Length: 0
	{
		var buf bytes.Buffer
		w := response.NewWriter(&buf)

		err := notFoundHandler(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "GET",
				RequestTarget: "/missing",
			},
		})
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "HTTP/1.1 404 Not Found\r\n")
		assert.Contains(t, out, "content-length: 0\r\n")
	}

	// Test: methodNotAllowedHandler writes Content-Length: 0
	{
		var buf bytes.Buffer
		w := response.NewWriter(&buf)

		err := methodNotAllowedHandler(w, &request.Request{
			RequestLine: request.RequestLine{
				Method:        "POST",
				RequestTarget: "/only-get",
			},
		})
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "HTTP/1.1 405 Method Not Allowed\r\n")
		assert.Contains(t, out, "content-length: 0\r\n")
	}
}
