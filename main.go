package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
)

func getLinesReader(c net.Conn) <- chan string {
	out := make(chan string, 1)

	go func() {
		defer c.Close()
		defer close(out)
		
		str := ""
		buf := make([]byte, 8)
		for {
			n, err := c.Read(buf)

			if n > 0 {
				data := buf[:n]
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

		go func(c net.Conn) {
			defer c.Close()
			for line := range getLinesReader(c) {
				fmt.Printf("%s\n", line)
			}
		}(conn)
	}
}
