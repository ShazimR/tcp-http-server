package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
)

func getLinesReader(f io.ReadCloser) <- chan string {
	out := make(chan string, 1)

	go func() {
		defer f.Close()
		defer close(out)
		
		str := ""
		for {
			data := make([]byte, 8)
			n, err := f.Read(data)

			if n > 0 {
				data = data[:n]
				for {
					i := bytes.IndexByte(data, '\n')
					if i == -1 {
						break
					}

					str += string(data[:i])
					out <- str
					str = ""

					data = data[i+1:]
				}

				str += string(data)
			}

			if err != nil {
				break
			}
		}

		if len(str) != 0 {
			out <- str
		}
	}()
	
	return out
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("error", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		for line := range getLinesReader(conn) {
			fmt.Printf("read: %s\n", line)
		}
	}
}
