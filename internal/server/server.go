package server

import (
	"fmt"
	"io"
	"net"

	"github.com/ShazimR/tcp-http-server/internal/response"
)

type Server struct {
	closed bool
}

func (s *Server) Close() error {
	s.closed = true
	return nil
}

func runConnection(_ *Server, conn io.ReadWriteCloser) {
	defer conn.Close()

	headers := response.GetDefaultHeaders(0)
	response.WriteStatusLine(conn, response.StatusOK)
	response.WriteHeaders(conn, headers)
}

func runServer(s *Server, listener net.Listener) {
	for {
		conn, err := listener.Accept()

		if s.closed {
			return
		}

		if err != nil {
			continue
		}

		go runConnection(s, conn)
	}
}

func Serve(port uint16) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	server := &Server{closed: false}
	go runServer(server, listener)

	return server, nil
}
