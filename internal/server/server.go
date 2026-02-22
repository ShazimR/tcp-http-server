package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/ShazimR/tcp-http-server/internal/router"
)

type Server struct {
	closed   atomic.Bool
	listener net.Listener
	handler  response.Handler
	router   *router.Router
}

func (s *Server) Close() error {
	s.closed.Store(true)
	return s.listener.Close()
}

func shouldClose(r *request.Request, w *response.Writer) bool {
	connHeader, _ := r.Headers.Get("Connection")

	if r.RequestLine.HttpVersion == "1.0" {
		close := connHeader != "keep-alive"
		if close {
			w.ForceCloseConnection()
		}
		return close
	}

	close := connHeader == "close"
	if close {
		w.ForceCloseConnection()
	}
	return close
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	responseWriter := response.NewWriter(conn)

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		r, err := request.RequestFromReader(conn)
		if errors.Is(err, request.ErrUnsupportedVersion) {
			body := []byte(err.Error())
			h := response.GetDefaultHeaders(len(body))
			_ = responseWriter.WriteResponse(response.StatusHttpVersionNotSupported, h, body)
			return
		}
		if errors.Is(err, request.ErrMalformedRequestLine) ||
			errors.Is(err, headers.ErrMalformedFieldLine) ||
			errors.Is(err, headers.ErrMalformedHeader) ||
			errors.Is(err, headers.ErrMalformedHeaderName) ||
			errors.Is(err, request.ErrMalformedChunkedBody) {
			body := []byte(err.Error())
			h := response.GetDefaultHeaders(len(body))
			_ = responseWriter.WriteResponse(response.StatusBadRequest, h, body)
			return
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			body := []byte(err.Error())
			h := response.GetDefaultHeaders(len(body))
			responseWriter.ForceCloseConnection()
			_ = responseWriter.WriteResponse(response.StatusRequestTimeout, h, body)
		}
		if err != nil {
			body := []byte(err.Error())
			h := response.GetDefaultHeaders(len(body))
			_ = responseWriter.WriteResponse(response.StatusInternalServerError, h, body)
			return
		}

		responseWriter.SetHttpVersion(r.RequestLine.HttpVersion)
		shouldClose := shouldClose(r, responseWriter)

		var handler response.Handler
		if s.handler != nil {
			handler = s.handler
		} else if s.router != nil {
			handler = s.router.GetHandler(r)
		} else {
			body := []byte("")
			h := response.GetDefaultHeaders(len(body))
			_ = responseWriter.WriteResponse(response.StatusInternalServerError, h, body)
			log.Printf("handler does not exist")
			return
		}

		err = handler(responseWriter, r)
		if errors.Is(err, syscall.EPIPE) ||
			errors.Is(err, syscall.ECONNRESET) ||
			errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			log.Printf("error from handler: %v\n", err)
			return
		}

		if shouldClose {
			return
		}
	}
}

func (s *Server) listen() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}
			log.Printf("error accepting connection %v", err)
			continue
		}

		go s.handle(conn)
	}
}

func Serve(port uint16, handler response.Handler, router *router.Router) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	server := &Server{
		closed:   atomic.Bool{},
		handler:  handler,
		router:   router,
		listener: listener,
	}

	go server.listen()
	return server, nil
}
