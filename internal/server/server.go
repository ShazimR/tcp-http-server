package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
)

type Handler func(w *response.Writer, req *request.Request)

type Server struct {
	closed   atomic.Bool
	listener net.Listener
	handler  Handler
}

func (s *Server) Close() error {
	s.closed.Store(true)
	return s.listener.Close()
}

func (s *Server) handle(conn io.ReadWriteCloser) {
	defer conn.Close()

	responseWriter := response.NewWriter(conn)
	r, err := request.RequestFromReader(conn)
	if err != nil {
		body := []byte(err.Error())
		h := response.GetDefaultHeaders(len(body))
		responseWriter.WriteStatusLine(response.StatusBadRequest)
		responseWriter.WriteHeaders(h)
		responseWriter.WriteBody(body)
		return
	}

	s.handler(responseWriter, r)
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

func Serve(port uint16, handler Handler) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	server := &Server{
		closed:   atomic.Bool{},
		handler:  handler,
		listener: listener,
	}

	go server.listen()
	return server, nil
}
