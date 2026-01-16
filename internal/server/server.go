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

type Handler func(w *response.Writer, req *request.Request) error

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
		if err := responseWriter.WriteResponse(response.StatusBadRequest, h, body); err != nil {
			return
		}

		return
	}

	err = s.handler(responseWriter, r)
	if err != nil {
		log.Printf("error from handler: %v\n", err)
		return
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
