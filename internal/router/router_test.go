package router

import (
	"bytes"
	"testing"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkReq(method, target string) *request.Request {
	return &request.Request{
		RequestLine: request.RequestLine{
			Method:        method,
			RequestTarget: target,
		},
		PathParams: make(map[string]string),
	}
}

func runHandler(t *testing.T, h response.Handler, req *request.Request) string {
	t.Helper()
	var buf bytes.Buffer
	w := response.NewWriter(&buf)

	err := h(w, req)
	require.NoError(t, err)

	return buf.String()
}

func TestRouter_StaticExactMatch(t *testing.T) {
	r := NewRouter()

	called := false
	okHandler := func(w *response.Writer, req *request.Request) error {
		called = true
		return nil
	}

	require.NoError(t, r.GET("/hello", okHandler))

	req := mkReq("GET", "/hello")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.True(t, called)
}

func TestRouter_NotFound(t *testing.T) {
	r := NewRouter()

	req := mkReq("GET", "/nope")
	h := r.GetHandler(req)
	out := runHandler(t, h, req)

	assert.Contains(t, out, "404")
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	r := NewRouter()

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, r.GET("/only-get", okHandler))

	req := mkReq("POST", "/only-get")
	h := r.GetHandler(req)
	out := runHandler(t, h, req)

	assert.Contains(t, out, "405")
}

func TestRouter_PathParamCaptured(t *testing.T) {
	r := NewRouter()

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, r.GET("/users/:id", okHandler))

	req := mkReq("GET", "/users/42")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.Equal(t, "42", req.PathParams["id"])
}

func TestRouter_StaticBeatsParam(t *testing.T) {
	r := NewRouter()

	var matched string

	staticHandler := func(w *response.Writer, req *request.Request) error {
		matched = "static"
		return nil
	}
	paramHandler := func(w *response.Writer, req *request.Request) error {
		matched = "param"
		return nil
	}

	require.NoError(t, r.GET("/users/me", staticHandler))
	require.NoError(t, r.GET("/users/:id", paramHandler))

	req := mkReq("GET", "/users/me")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.Equal(t, "static", matched)
}

func TestRouter_MultiParamRoute(t *testing.T) {
	r := NewRouter()

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, r.GET("/users/:id/posts/:postId", okHandler))

	req := mkReq("GET", "/users/7/posts/99")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.Equal(t, "7", req.PathParams["id"])
	assert.Equal(t, "99", req.PathParams["postId"])
}

func TestRouter_AmbiguousParamNamesRejected(t *testing.T) {
	r := NewRouter()

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }

	require.NoError(t, r.GET("/users/:id", okHandler))
	err := r.GET("/users/:userId", okHandler)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAmbiguousPathParams)
}

func TestRouter_RootRoute(t *testing.T) {
	r := NewRouter()

	called := false
	rootHandler := func(w *response.Writer, req *request.Request) error {
		called = true
		return nil
	}

	require.NoError(t, r.GET("/", rootHandler))

	req := mkReq("GET", "/")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.True(t, called)
}

func TestRouter_ManyRoutes_StaticAndParamBranches(t *testing.T) {
	r := NewRouter()

	noop := func(w *response.Writer, req *request.Request) error { return nil }

	// Register a bunch of static routes
	staticRoutes := []string{
		"/",
		"/health",
		"/users",
		"/users/me",
		"/users/settings",
		"/users/settings/privacy",
		"/posts",
		"/posts/recent",
		"/assets/app.js",
		"/assets/styles.css",
		"/api/v1/ping",
		"/api/v1/users",
		"/api/v1/users/me",
		"/api/v1/users/settings",
		"/api/v1/posts",
		"/api/v1/posts/recent",
	}

	for _, p := range staticRoutes {
		require.NoError(t, r.GET(p, noop), "failed to add GET %s", p)
	}

	// Register param routes that overlap with statics
	require.NoError(t, r.GET("/users/:id", noop))
	require.NoError(t, r.GET("/users/:id/posts", noop))
	require.NoError(t, r.GET("/users/:id/posts/:postId", noop))
	require.NoError(t, r.GET("/api/v1/users/:id", noop))
	require.NoError(t, r.GET("/api/v1/posts/:id", noop))

	// Ensure a bunch of lookups work and don't confuse branches
	cases := []struct {
		method    string
		target    string
		want404   bool
		wantParam map[string]string
	}{
		{"GET", "/", false, nil},
		{"GET", "/health", false, nil},
		{"GET", "/users/me", false, nil}, // must hit static, not param
		{"GET", "/users/123", false, map[string]string{"id": "123"}},
		{"GET", "/users/123/posts", false, map[string]string{"id": "123"}},
		{"GET", "/users/123/posts/999", false, map[string]string{"id": "123", "postId": "999"}},
		{"GET", "/api/v1/users/me", false, nil}, // must hit static, not param
		{"GET", "/api/v1/users/77", false, map[string]string{"id": "77"}},
		{"GET", "/api/v1/posts/888", false, map[string]string{"id": "888"}},
		{"GET", "/does-not-exist", true, nil},
		{"GET", "/users/123/does-not-exist", true, nil},
	}

	for _, tc := range cases {
		req := mkReq(tc.method, tc.target)
		h := r.GetHandler(req)
		out := runHandler(t, h, req)

		if tc.want404 {
			assert.Contains(t, out, "404", "expected 404 for %s %s", tc.method, tc.target)
			continue
		}
		assert.NotContains(t, out, "404", "unexpected 404 for %s %s", tc.method, tc.target)

		for k, v := range tc.wantParam {
			assert.Equal(t, v, req.PathParams[k], "wrong param for %s %s", tc.method, tc.target)
		}
	}
}

func TestRouter_StaticBeatsParam_MultipleDepths(t *testing.T) {
	r := NewRouter()

	var hit string
	static := func(name string) response.Handler {
		return func(w *response.Writer, req *request.Request) error {
			hit = name
			return nil
		}
	}

	noop := func(w *response.Writer, req *request.Request) error { return nil }

	// Level 1 overlap
	require.NoError(t, r.GET("/users/me", static("users/me")))
	require.NoError(t, r.GET("/users/:id", noop))

	// Level 2 overlap
	require.NoError(t, r.GET("/users/me/posts", static("users/me/posts")))
	require.NoError(t, r.GET("/users/:id/posts", noop))

	// Level 3 overlap
	require.NoError(t, r.GET("/users/me/posts/recent", static("users/me/posts/recent")))
	require.NoError(t, r.GET("/users/:id/posts/:postId", noop))

	req := mkReq("GET", "/users/me")
	_ = runHandler(t, r.GetHandler(req), req)
	assert.Equal(t, "users/me", hit)

	req = mkReq("GET", "/users/me/posts")
	_ = runHandler(t, r.GetHandler(req), req)
	assert.Equal(t, "users/me/posts", hit)

	req = mkReq("GET", "/users/me/posts/recent")
	_ = runHandler(t, r.GetHandler(req), req)
	assert.Equal(t, "users/me/posts/recent", hit)
}

func TestRouter_MethodNotAllowed_OnParamMatch(t *testing.T) {
	r := NewRouter()

	getOnly := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, r.GET("/users/:id", getOnly))

	req := mkReq("POST", "/users/123")
	h := r.GetHandler(req)
	out := runHandler(t, h, req)

	assert.Contains(t, out, "405")
}

func TestRouter_AmbiguousParamsRejected_Deeper(t *testing.T) {
	r := NewRouter()
	noop := func(w *response.Writer, req *request.Request) error { return nil }

	require.NoError(t, r.GET("/users/:id/posts/:postId", noop))
	err := r.GET("/users/:id/posts/:pid", noop) // same position, different param name

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAmbiguousPathParams)
}

func TestRouter_AddRouteValidation(t *testing.T) {
	r := NewRouter()
	noop := func(w *response.Writer, req *request.Request) error { return nil }

	// empty path
	err := r.GET("", noop)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequestTargetEmpty)

	// malformed (no leading slash)
	err = r.GET("users", noop)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedRequestTarget)
}
