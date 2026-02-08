package router

import (
	"testing"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helpers
func mwTag(tag string, order *[]string) Middleware {
	return func(next response.Handler) response.Handler {
		return func(w *response.Writer, req *request.Request) error {
			*order = append(*order, tag)
			return next(w, req)
		}
	}
}

func mwShortCircuit(body string) Middleware {
	return func(next response.Handler) response.Handler {
		return func(w *response.Writer, req *request.Request) error {
			return w.WriteBody([]byte(body))
		}
	}
}

// Tests
func TestMiddleware_RunsBeforeHandler(t *testing.T) {
	r := NewRouter()

	var order []string

	r.Use(mwTag("mw", &order))

	handler := func(w *response.Writer, req *request.Request) error {
		order = append(order, "handler")
		return nil
	}

	require.NoError(t, r.GET("/ping", handler))

	req := mkReq("GET", "/ping")
	_ = runHandler(t, r.GetHandler(req), req)

	assert.Equal(t, []string{"mw", "handler"}, order)
}

func TestMiddleware_MultipleExecutionOrder(t *testing.T) {
	r := NewRouter()

	var order []string

	r.Use(
		mwTag("mw1", &order),
		mwTag("mw2", &order),
	)

	handler := func(w *response.Writer, req *request.Request) error {
		order = append(order, "handler")
		return nil
	}

	require.NoError(t, r.GET("/ping", handler))

	req := mkReq("GET", "/ping")
	_ = runHandler(t, r.GetHandler(req), req)

	assert.Equal(t, []string{"mw1", "mw2", "handler"}, order)
}

func TestMiddleware_ShortCircuit(t *testing.T) {
	r := NewRouter()

	r.Use(mwShortCircuit("blocked"))

	handlerCalled := false
	handler := func(w *response.Writer, req *request.Request) error {
		handlerCalled = true
		return nil
	}

	require.NoError(t, r.GET("/ping", handler))

	req := mkReq("GET", "/ping")
	out := runHandler(t, r.GetHandler(req), req)

	assert.False(t, handlerCalled)
	assert.Contains(t, out, "blocked")
}

func TestMiddleware_GroupSpecific(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	var order []string

	api.Use(mwTag("api-mw", &order))

	handler := func(w *response.Writer, req *request.Request) error {
		order = append(order, "handler")
		return nil
	}

	require.NoError(t, api.GET("/ping", handler))

	req := mkReq("GET", "/api/ping")
	_ = runHandler(t, r.GetHandler(req), req)

	assert.Equal(t, []string{"api-mw", "handler"}, order)
}

func TestMiddleware_RouterAndGroupCombinedOrder(t *testing.T) {
	var order []string
	r := NewRouter()
	r.Use(mwTag("root-mw", &order))
	api := r.Group("/api")
	api.Use(mwTag("api-mw", &order))

	handler := func(w *response.Writer, req *request.Request) error {
		order = append(order, "handler")
		return nil
	}

	require.NoError(t, api.GET("/ping", handler))

	req := mkReq("GET", "/api/ping")
	_ = runHandler(t, r.GetHandler(req), req)

	assert.Equal(t,
		[]string{"root-mw", "api-mw", "handler"},
		order,
	)
}
