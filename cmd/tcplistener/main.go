package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/ShazimR/tcp-http-server/internal/request"
)

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("error", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("error: %e", err)
			continue
		}

		go func(c io.ReadCloser) {
			defer c.Close()

			r, err := request.RequestFromReader(c)
			if err != nil {
				fmt.Printf("error: %e", err)
				return
			}

			fmt.Printf("Request Line:\n")
			fmt.Printf("- Method:  %s\n", r.RequestLine.Method)
			fmt.Printf("- Target:  %s\n", r.RequestLine.RequestTarget)
			fmt.Printf("- Version: %s\n", r.RequestLine.HttpVersion)
		}(conn)
	}
}
