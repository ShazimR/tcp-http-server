package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/ShazimR/tcp-http-server/internal/router"
)

type ServerConfig struct {
	Port        uint16
	Handler     response.Handler
	Router      *router.Router
	ReadTimeout time.Duration
	Silent      bool
}

type Server struct {
	closed      atomic.Bool
	readTimeout time.Duration
	listener    net.Listener
	handler     response.Handler
	router      *router.Router
	silent      bool
}

func (s *Server) Close() error {
	s.closed.Store(true)
	return s.listener.Close()
}

func shouldClose(r *request.Request, w *response.Writer) bool {
	connHeader, _ := r.Headers.Get("Connection")
	connHeader = strings.ToLower(connHeader)

	if r.RequestLine.HttpVersion == "1.0" {
		close := connHeader != "keep-alive"
		if close {
			w.ForceCloseConnection()
		}
		return close
	}

	close := strings.Contains(connHeader, "close")
	if close {
		w.ForceCloseConnection()
	}
	return close
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	responseWriter := response.NewWriter(conn)

	for {
		currentTime := time.Now()
		currentTimeStr := currentTime.Format(time.RFC1123)
		conn.SetReadDeadline(currentTime.Add(s.readTimeout))
		responseWriter.ResetLogs()

		r, err := request.RequestFromReader(conn)
		if errors.Is(err, request.ErrUnsupportedVersion) {
			status := response.StatusHttpVersionNotSupported
			body := []byte("expected HTTP/1.0 or HTTP/1.1")
			h := response.GetDefaultHeaders(len(body))
			_ = responseWriter.WriteResponse(status, h, body)
			if !s.silent {
				log.Printf("%s INFO: Request had unsupported version (%v); Returned status %d\n\n", currentTimeStr, err, status)
			}
			return

		} else if errors.Is(err, request.ErrMalformedRequestLine) ||
			errors.Is(err, headers.ErrMalformedFieldLine) ||
			errors.Is(err, headers.ErrMalformedHeader) ||
			errors.Is(err, headers.ErrMalformedHeaderName) ||
			errors.Is(err, request.ErrMalformedChunkedBody) {
			status := response.StatusBadRequest
			body := []byte("")
			h := response.GetDefaultHeaders(len(body))
			_ = responseWriter.WriteResponse(status, h, body)
			if !s.silent {
				log.Printf("%s INFO: Request was bad (%v); Returned status %d\n\n", currentTimeStr, err, status)
			}
			return

		} else if errors.Is(err, os.ErrDeadlineExceeded) {
			status := response.StatusRequestTimeout
			body := []byte("connection timed out")
			h := response.GetDefaultHeaders(len(body))
			responseWriter.ForceCloseConnection()
			_ = responseWriter.WriteResponse(status, h, body)
			if !s.silent {
				log.Printf("%s INFO: Request from client timed out (%v); Returned status %d; Closing connection\n\n", currentTimeStr, err, status)
			}
			return

		} else if errors.Is(err, syscall.EPIPE) ||
			errors.Is(err, syscall.ECONNRESET) ||
			errors.Is(err, io.EOF) ||
			errors.Is(err, net.ErrClosed) {
			if !s.silent {
				log.Printf("%s INFO: Client disconnected while reading request (%v); Closing connection\n\n", currentTimeStr, err)
			}
			return

		} else if err != nil {
			status := response.StatusInternalServerError
			body := []byte("failed to parse request")
			h := response.GetDefaultHeaders(len(body))
			responseWriter.ForceCloseConnection()
			_ = responseWriter.WriteResponse(status, h, body)
			if !s.silent {
				log.Printf("%s ERROR: Internal server error (%v); Returned status %d; Closing connection\n\n", currentTimeStr, err, status)
			}
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
			status := response.StatusInternalServerError
			body := []byte("")
			h := response.GetDefaultHeaders(len(body))
			responseWriter.ForceCloseConnection()
			_ = responseWriter.WriteResponse(status, h, body)
			if !s.silent {
				log.Printf("%s ERROR: No handler found for request; Returned status %d\n\n; Closing connection", currentTimeStr, status)
			}
			return
		}

		err = handler(responseWriter, r)
		if errors.Is(err, syscall.EPIPE) ||
			errors.Is(err, syscall.ECONNRESET) ||
			errors.Is(err, io.EOF) ||
			errors.Is(err, net.ErrClosed) {
			if !s.silent {
				log.Printf("%s INFO: Client disconnected while writing response (%v); Closing connection\n\n", currentTimeStr, err)
			}
			return
		}
		if err != nil {
			// Unknown state of response -> force close without sending a response
			if !s.silent {
				log.Printf("%s ERROR: Handler Error (%v)\n\n; Closing connection", currentTimeStr, err)
			}
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
			if !s.silent {
				log.Printf("%s ERROR: Failed to accept connection (%v)\n\n", time.Now().Format(time.RFC1123), err)
			}
			continue
		}

		go s.handle(conn)
	}
}

func Serve(port uint16, handler response.Handler, router *router.Router) (*Server, error) {
	return ServeWithConfig(&ServerConfig{
		Port:        port,
		ReadTimeout: 30 * time.Second,
		Handler:     handler,
		Router:      router,
		Silent:      false,
	})
}

func ServeWithConfig(config *ServerConfig) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.Port))
	if err != nil {
		return nil, err
	}

	readTimeout := config.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}

	server := &Server{
		closed:      atomic.Bool{},
		readTimeout: readTimeout,
		handler:     config.Handler,
		router:      config.Router,
		silent:      config.Silent,
		listener:    listener,
	}

	go server.listen()
	return server, nil
}
