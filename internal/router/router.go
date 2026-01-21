package router

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
)

type Method uint

const (
	GET Method = iota
	POST
	PUT
	DELETE
	PATCH
	HEAD
	CONNECT
	OPTIONS
	TRACE
	MethodCount
)

type Router struct {
	routes [MethodCount]map[string]response.Handler
}

func NewRouter() *Router {
	routes := [MethodCount]map[string]response.Handler{}
	for i := range routes {
		routes[i] = make(map[string]response.Handler)
	}
	return &Router{routes: routes}
}

func (r *Router) GET(path string, handler response.Handler) {
	r.routes[GET][getNormalizedPath(path)] = handler
}

func (r *Router) POST(path string, handler response.Handler) {
	r.routes[POST][getNormalizedPath(path)] = handler
}

func (r *Router) PUT(path string, handler response.Handler) {
	r.routes[PUT][getNormalizedPath(path)] = handler
}

func (r *Router) DELETE(path string, handler response.Handler) {
	r.routes[DELETE][getNormalizedPath(path)] = handler
}

func (r *Router) PATCH(path string, handler response.Handler) {
	r.routes[PATCH][getNormalizedPath(path)] = handler
}

func (r *Router) HEAD(path string, handler response.Handler) {
	r.routes[HEAD][getNormalizedPath(path)] = handler
}

func (r *Router) CONNECT(path string, handler response.Handler) {
	r.routes[CONNECT][getNormalizedPath(path)] = handler
}

func (r *Router) OPTIONS(path string, handler response.Handler) {
	r.routes[OPTIONS][getNormalizedPath(path)] = handler
}

func (r *Router) TRACE(path string, handler response.Handler) {
	r.routes[TRACE][getNormalizedPath(path)] = handler
}

func (r *Router) GetHandler(req *request.Request) response.Handler {
	method := getMethod(req.RequestLine.Method)
	path := getNormalizedPath(req.RequestLine.RequestTarget)

	if method < MethodCount {
		m := r.routes[method]
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

func getNormalizedPath(path string) string {
	i := strings.IndexByte(path, '?')
	if i == -1 {
		return path
	}
	if i == len(path)-1 {
		return path[:i]
	}

	normalizedPath := path[:i]
	params := []string{}
	queryStr := path[i+1:]
	queries := strings.Split(queryStr, "&")

	for _, query := range queries {
		if k, _, hasEq := strings.Cut(query, "="); hasEq || k != "" {
			params = append(params, k)
		}
	}

	slices.Sort(params)

	for _, param := range params {
		normalizedPath += fmt.Sprintf("&%s", param)
	}

	return normalizedPath
}

func getMethod(s string) Method {
	var method Method

	switch s {
	case "GET":
		method = GET
	case "POST":
		method = POST
	case "PUT":
		method = PUT
	case "DELETE":
		method = DELETE
	case "PATCH":
		method = PATCH
	case "HEAD":
		method = HEAD
	case "CONNECT":
		method = CONNECT
	case "OPTIONS":
		method = OPTIONS
	case "TRACE":
		method = TRACE
	default:
		method = MethodCount
	}

	return method
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
