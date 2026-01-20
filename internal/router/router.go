package router

import (
	"strings"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
)

type Router struct {
	routes map[string]map[string]response.Handler
}

func NewRouter() *Router {
	return &Router{
		routes: map[string]map[string]response.Handler{
			"GET":     make(map[string]response.Handler),
			"POST":    make(map[string]response.Handler),
			"PUT":     make(map[string]response.Handler),
			"DELETE":  make(map[string]response.Handler),
			"PATCH":   make(map[string]response.Handler),
			"HEAD":    make(map[string]response.Handler),
			"CONNECT": make(map[string]response.Handler),
			"OPTIONS": make(map[string]response.Handler),
			"TRACE":   make(map[string]response.Handler),
		},
	}
}

func (r *Router) GET(path string, handler response.Handler) {
	r.routes["GET"][path] = handler
}

func (r *Router) POST(path string, handler response.Handler) {
	r.routes["POST"][path] = handler
}

func (r *Router) PUT(path string, handler response.Handler) {
	r.routes["PUT"][path] = handler
}

func (r *Router) DELETE(path string, handler response.Handler) {
	r.routes["DELETE"][path] = handler
}

func (r *Router) PATCH(path string, handler response.Handler) {
	r.routes["PATCH"][path] = handler
}

func (r *Router) HEAD(path string, handler response.Handler) {
	r.routes["HEAD"][path] = handler
}

func (r *Router) CONNECT(path string, handler response.Handler) {
	r.routes["CONNECT"][path] = handler
}

func (r *Router) OPTIONS(path string, handler response.Handler) {
	r.routes["OPTIONS"][path] = handler
}

func (r *Router) TRACE(path string, handler response.Handler) {
	r.routes["TRACE"][path] = handler
}

func (r *Router) GetHandler(req *request.Request) response.Handler {
	method := req.RequestLine.Method
	// TODO: Handle query strings
	path := strings.Split(req.RequestLine.RequestTarget, "?")[0]

	if m, ok := r.routes[method]; ok {
		if h, ok := m[path]; ok {
			return h
		}
	}

	for _, m := range r.routes {
		if _, ok := m[path]; ok {
			return methodNotAllowedHandler
		}
	}

	return notFoundHandler
}

func methodNotAllowedHandler(w *response.Writer, req *request.Request) error {
	status := response.StatusMethodNotAllowed
	h := response.GetDefaultHeaders(0)
	body := []byte("")
	if err := w.WriteResponse(status, h, body); err != nil {
		return err
	}

	return nil
}

func notFoundHandler(w *response.Writer, req *request.Request) error {
	status := response.StatusNotFound
	h := response.GetDefaultHeaders(0)
	body := []byte("")
	if err := w.WriteResponse(status, h, body); err != nil {
		return err
	}

	return nil
}
