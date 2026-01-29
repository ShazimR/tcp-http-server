package router

import (
	"testing"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterGroup_StaticExactMatch(t *testing.T) {
	r := NewRouter()

	api := r.Group("/api")

	called := false
	okHandler := func(w *response.Writer, req *request.Request) error {
		called = true
		return nil
	}

	require.NoError(t, api.GET("/hello", okHandler))

	req := mkReq("GET", "/api/hello")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.True(t, called)
}

func TestRouterGroup_NotFound(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	noop := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, api.GET("/exists", noop))

	req := mkReq("GET", "/api/nope")
	h := r.GetHandler(req)
	out := runHandler(t, h, req)

	assert.Contains(t, out, "404")
}

func TestRouterGroup_MethodNotAllowed(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, api.GET("/only-get", okHandler))

	req := mkReq("POST", "/api/only-get")
	h := r.GetHandler(req)
	out := runHandler(t, h, req)

	assert.Contains(t, out, "405")
}

func TestRouterGroup_NestedGroups(t *testing.T) {
	r := NewRouter()

	api := r.Group("/api")
	v1 := api.Group("/v1")

	called := false
	okHandler := func(w *response.Writer, req *request.Request) error {
		called = true
		return nil
	}

	require.NoError(t, v1.GET("/ping", okHandler))

	req := mkReq("GET", "/api/v1/ping")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.True(t, called)
}

func TestRouterGroup_PathParamCaptured(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, api.GET("/users/:id", okHandler))

	req := mkReq("GET", "/api/users/42")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.Equal(t, "42", req.PathParams["id"])
}

func TestRouterGroup_MultiParamRoute(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, api.GET("/users/:id/posts/:postId", okHandler))

	req := mkReq("GET", "/api/users/7/posts/99")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.Equal(t, "7", req.PathParams["id"])
	assert.Equal(t, "99", req.PathParams["postId"])
}

func TestRouterGroup_StaticBeatsParam(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	var matched string

	staticHandler := func(w *response.Writer, req *request.Request) error {
		matched = "static"
		return nil
	}
	paramHandler := func(w *response.Writer, req *request.Request) error {
		matched = "param"
		return nil
	}

	require.NoError(t, api.GET("/users/me", staticHandler))
	require.NoError(t, api.GET("/users/:id", paramHandler))

	req := mkReq("GET", "/api/users/me")
	h := r.GetHandler(req)
	_ = runHandler(t, h, req)

	assert.Equal(t, "static", matched)
}

func TestRouterGroup_AmbiguousParamNamesRejected(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	okHandler := func(w *response.Writer, req *request.Request) error { return nil }

	require.NoError(t, api.GET("/users/:id", okHandler))
	err := api.GET("/users/:userId", okHandler)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAmbiguousPathParams)
}

func TestRouterGroup_RootRouteInsideGroup(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	called := false
	rootHandler := func(w *response.Writer, req *request.Request) error {
		called = true
		return nil
	}

	require.NoError(t, api.GET("/", rootHandler))

	// With trailing-slash normalization, both should match:
	req1 := mkReq("GET", "/api")
	_ = runHandler(t, r.GetHandler(req1), req1)
	assert.True(t, called)

	called = false
	req2 := mkReq("GET", "/api/")
	_ = runHandler(t, r.GetHandler(req2), req2)
	assert.True(t, called)
}

func TestRouterGroup_MethodNotAllowed_OnParamMatch(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")

	getOnly := func(w *response.Writer, req *request.Request) error { return nil }
	require.NoError(t, api.GET("/users/:id", getOnly))

	req := mkReq("POST", "/api/users/123")
	h := r.GetHandler(req)
	out := runHandler(t, h, req)

	assert.Contains(t, out, "405")
}

func TestRouter_GroupedSameEndpointDifferentHandlers(t *testing.T) {
	r := NewRouter()

	api := r.Group("/api")
	private := r.Group("/private")

	apiPing := func(w *response.Writer, req *request.Request) error {
		return w.WriteBody([]byte("api-ping"))
	}
	privatePing := func(w *response.Writer, req *request.Request) error {
		return w.WriteBody([]byte("private-ping"))
	}

	require.NoError(t, api.GET("/ping", apiPing))
	require.NoError(t, private.GET("/ping", privatePing))

	// /api/ping should hit api handler
	req := mkReq("GET", "/api/ping")
	out := runHandler(t, r.GetHandler(req), req)
	assert.Contains(t, out, "api-ping")
	assert.NotContains(t, out, "private-ping")

	// /private/ping should hit private handler
	req = mkReq("GET", "/private/ping")
	out = runHandler(t, r.GetHandler(req), req)
	assert.Contains(t, out, "private-ping")
	assert.NotContains(t, out, "api-ping")
}

func TestRouter_RootGroupBehavesLikeRouter(t *testing.T) {
	r := NewRouter()
	root := r.Group("/")

	hit := false
	handler := func(w *response.Writer, req *request.Request) error {
		hit = true
		return nil
	}

	require.NoError(t, root.GET("/ping", handler))

	req := mkReq("GET", "/ping")
	_ = runHandler(t, r.GetHandler(req), req)

	assert.True(t, hit)
}

func TestRouterGroup_ManyRoutes_StaticAndParamBranches(t *testing.T) {
	r := NewRouter()

	api := r.Group("/api")
	v1 := api.Group("/v1")

	noop := func(w *response.Writer, req *request.Request) error { return nil }

	// Register a bunch of static routes (grouped)
	staticRoutes := []struct {
		group *Router
		path  string
	}{
		{api, "/"},
		{api, "/health"},
		{api, "/users"},
		{api, "/users/me"},
		{api, "/users/settings"},
		{api, "/users/settings/privacy"},
		{api, "/posts"},
		{api, "/posts/recent"},
		{api, "/assets/app.js"},
		{api, "/assets/styles.css"},
		{v1, "/ping"},
		{v1, "/users"},
		{v1, "/users/me"},
		{v1, "/users/settings"},
		{v1, "/posts"},
		{v1, "/posts/recent"},
	}

	for _, sr := range staticRoutes {
		require.NoError(t, sr.group.GET(sr.path, noop), "failed to add GET %s", sr.path)
	}

	// Register param routes that overlap with statics (grouped)
	require.NoError(t, api.GET("/users/:id", noop))
	require.NoError(t, api.GET("/users/:id/posts", noop))
	require.NoError(t, api.GET("/users/:id/posts/:postId", noop))
	require.NoError(t, v1.GET("/users/:id", noop))
	require.NoError(t, v1.GET("/posts/:id", noop))

	// Ensure a bunch of lookups work and don't confuse branches
	cases := []struct {
		method    string
		target    string
		want404   bool
		wantParam map[string]string
	}{
		{"GET", "/api", false, nil},
		{"GET", "/api/health", false, nil},
		{"GET", "/api/users/me", false, nil}, // must hit static, not param
		{"GET", "/api/users/123", false, map[string]string{"id": "123"}},
		{"GET", "/api/users/123/posts", false, map[string]string{"id": "123"}},
		{"GET", "/api/users/123/posts/999", false, map[string]string{"id": "123", "postId": "999"}},
		{"GET", "/api/v1/users/me", false, nil}, // must hit static, not param
		{"GET", "/api/v1/users/77", false, map[string]string{"id": "77"}},
		{"GET", "/api/v1/posts/888", false, map[string]string{"id": "888"}},
		{"GET", "/api/does-not-exist", true, nil},
		{"GET", "/api/users/123/does-not-exist", true, nil},
		{"GET", "/api/v1/does-not-exist", true, nil},
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
