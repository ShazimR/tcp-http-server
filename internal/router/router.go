package router

import (
	"fmt"
	"strings"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
)

type method uint

const (
	methodGET method = iota
	methodPOST
	methodPUT
	methodDELETE
	methodPATCH
	methodCount
)

var (
	ErrInvalidHttpMethod      = fmt.Errorf("invalid http method")
	ErrRequestTargetEmpty     = fmt.Errorf("request target is empty")
	ErrMalformedRequestTarget = fmt.Errorf("malformed request target")
	ErrAmbiguousPathParams    = fmt.Errorf("added ambiguous path params")
)

type routerNode struct {
	token    string
	isParam  bool
	children []*routerNode
	handlers [methodCount]response.Handler
}

func newRouterNode(token string, isParam bool) *routerNode {
	return &routerNode{
		token:    token,
		isParam:  isParam,
		children: []*routerNode{},
		handlers: [methodCount]response.Handler{},
	}
}

func (node *routerNode) addChild(child *routerNode) {
	node.children = append(node.children, child)
}

func (node *routerNode) setMethodHandler(m method, handler response.Handler) error {
	if m >= methodCount {
		return ErrInvalidHttpMethod
	}

	node.handlers[m] = handler
	return nil
}

func (node *routerNode) getStaticChild(token string) *routerNode {
	for _, child := range node.children {
		if !child.isParam && child.token == token {
			return child
		}
	}

	return nil
}

func (node *routerNode) getParamChild() *routerNode {
	for _, child := range node.children {
		if child.isParam {
			return child
		}
	}

	return nil
}

func (node *routerNode) matchChild(token string) (child *routerNode, usedParam bool) {
	if c := node.getStaticChild(token); c != nil {
		return c, false
	}

	if c := node.getParamChild(); c != nil {
		return c, true
	}

	return nil, false
}

func (node *routerNode) getHandler(m method) (response.Handler, error) {
	if m >= methodCount {
		return notFoundHandler, ErrInvalidHttpMethod
	}

	return node.handlers[m], nil
}

type Router struct {
	routes *routerNode
	prefix string
}

func NewRouter() *Router {
	head := newRouterNode("/", false)
	return &Router{
		routes: head,
		prefix: "",
	}
}

func (r *Router) withPrefix(path string) (string, error) {
	if r.prefix == "" {
		return path, nil
	}
	if path == "/" || path == "" {
		return r.prefix, nil
	}
	if path[0] != '/' {
		return path, ErrMalformedRequestTarget
	}

	return (r.prefix + path), nil
}

func (r *Router) addRoute(tokens []string, m method, handler response.Handler) error {
	runner := r.routes
	for _, token := range tokens {
		isParam := len(token) > 0 && token[0] == ':'

		var node *routerNode
		if isParam {
			node = runner.getParamChild()
			if node == nil {
				node = newRouterNode(token[1:], true)
				runner.addChild(node)

			} else if node.token != token[1:] {
				return ErrAmbiguousPathParams
			}

		} else {
			node = runner.getStaticChild(token)
			if node == nil {
				node = newRouterNode(token, false)
				runner.addChild(node)
			}
		}

		runner = node
	}

	return runner.setMethodHandler(m, handler)
}

func (r *Router) GET(path string, handler response.Handler) error {
	fullPath, err := r.withPrefix(path)
	if err != nil {
		return err
	}

	tokens, err := getTokens(fullPath)
	if err != nil {
		return err
	}

	return r.addRoute(tokens, methodGET, handler)
}

func (r *Router) POST(path string, handler response.Handler) error {
	fullPath, err := r.withPrefix(path)
	if err != nil {
		return err
	}

	tokens, err := getTokens(fullPath)
	if err != nil {
		return err
	}

	return r.addRoute(tokens, methodPOST, handler)
}

func (r *Router) PUT(path string, handler response.Handler) error {
	fullPath, err := r.withPrefix(path)
	if err != nil {
		return err
	}

	tokens, err := getTokens(fullPath)
	if err != nil {
		return err
	}

	return r.addRoute(tokens, methodPUT, handler)
}

func (r *Router) DELETE(path string, handler response.Handler) error {
	fullPath, err := r.withPrefix(path)
	if err != nil {
		return err
	}

	tokens, err := getTokens(fullPath)
	if err != nil {
		return err
	}

	return r.addRoute(tokens, methodDELETE, handler)
}

func (r *Router) PATCH(path string, handler response.Handler) error {
	fullPath, err := r.withPrefix(path)
	if err != nil {
		return err
	}

	tokens, err := getTokens(fullPath)
	if err != nil {
		return err
	}

	return r.addRoute(tokens, methodPATCH, handler)
}

func (r *Router) Group(prefix string) *Router {
	var newPrefix string
	if prefix == "/" || prefix == "" {
		newPrefix = ""

	} else if !strings.HasPrefix(prefix, "/") {
		newPrefix = "/" + strings.TrimSuffix(prefix, "/")

	} else {
		newPrefix = strings.TrimSuffix(prefix, "/")
	}

	return &Router{
		routes: r.routes,
		prefix: r.prefix + newPrefix,
	}
}

func (r *Router) GetHandler(req *request.Request) response.Handler {
	m := getMethod(req.RequestLine.Method)
	if m >= methodCount {
		return notFoundHandler
	}

	tokens, err := getTokens(req.RequestLine.RequestTarget)
	if err != nil {
		return notFoundHandler
	}

	runner := r.routes
	for _, token := range tokens {
		node, usedParam := runner.matchChild(token)
		if node == nil {
			return notFoundHandler
		}

		if usedParam {
			req.PathParams[node.token] = token
		}

		runner = node
	}

	handler, err := runner.getHandler(m)
	if err != nil {
		return notFoundHandler
	}

	if handler == nil {
		for _, h := range runner.handlers {
			if h != nil {
				return methodNotAllowedHandler
			}
		}

		return notFoundHandler
	}

	return handler
}

func getTokens(path string) ([]string, error) {
	if len(path) == 0 {
		return nil, ErrRequestTargetEmpty
	}
	if path[0] != '/' {
		return nil, ErrMalformedRequestTarget
	}
	if path == "/" {
		return []string{}, nil
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return strings.Split(path, "/"), nil
}

func getMethod(s string) method {
	var m method

	switch s {
	case "GET":
		m = methodGET
	case "POST":
		m = methodPOST
	case "PUT":
		m = methodPUT
	case "DELETE":
		m = methodDELETE
	case "PATCH":
		m = methodPATCH
	default:
		m = methodCount
	}

	return m
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
