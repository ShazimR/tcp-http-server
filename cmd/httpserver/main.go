package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/ShazimR/tcp-http-server/internal/server"
)

const port = 8080

func respond200() []byte {
	return []byte(`<html>
<head>
    <title>200 OK</title>
</head>
<body>
    <h1>Success!</h1>
    <p>Your request was an absolute banger.</p>
</body>
</html>`)
}

func respond400() []byte {
	return []byte(`<html>
<head>
    <title>400 Bad Request</title>
</head>
<body>
    <h1>Bad Request</h1>
    <p>Your request honestly kinda sucked.</p>
</body>
</html>`)
}

func respond404() []byte {
	return []byte(`<html>
<head>
    <title>404 Not Found</title>
</head>
<body>
    <h1>Not Found</h1>
    <p>This page does not exist.</p>
</body>
</html>`)
}

func respond500() []byte {
	return []byte(`<html>
<head>
    <title>500 Internal Server Error</title>
</head>
<body>
    <h1>Internal Server Error</h1>
    <p>Okay, you know what? This one is on me.</p>
</body>
</html>`)
}

// Example handler for the server (must handle routing) until
// a router is implemented
func handler(w *response.Writer, req *request.Request) error {
	var status response.StatusCode
	var body []byte
	h := response.GetDefaultHeaders(0)
	chunked := false

	if req.RequestLine.RequestTarget == "/" {
		status = response.StatusOK
		body = respond200()

	} else if req.RequestLine.RequestTarget == "/yourproblem" {
		status = response.StatusBadRequest
		body = respond400()

	} else if req.RequestLine.RequestTarget == "/myproblem" {
		status = response.StatusInternalServerError
		body = respond500()

	} else if strings.HasPrefix(req.RequestLine.RequestTarget, "/static/") {
		target := req.RequestLine.RequestTarget[len("/static/"):]
		if len(target) == 0 {
			target = "index.html"
		}

		f, err := os.Open("./static/" + target)
		if errors.Is(err, os.ErrNotExist) {
			status = response.StatusNotFound
			body = respond404()

		} else if err != nil {
			status = response.StatusInternalServerError
			body = respond500()

		} else {
			defer f.Close()
			chunked = true

			if err = w.WriteStatusLine(response.StatusOK); err != nil {
				return err
			}
			h.Delete("Content-Length")
			h.Set("Transfer-Encoding", "chunked")
			h.Replace("Content-Type", "text/html")
			h.Set("Trailer", "X-Content-SHA256")
			h.Set("Trailer", "X-Content-Length")

			if err = w.WriteHeaders(h); err != nil {
				return err
			}

			fullBody := []byte{}
			data := make([]byte, 32)
			for {
				n, rErr := f.Read(data)
				if n > 0 {
					fullBody = append(fullBody, data[:n]...)

					if err = w.WriteBody(fmt.Appendf(nil, "%x\r\n", n)); err != nil {
						return err
					}
					if err = w.WriteBody(data[:n]); err != nil {
						return err
					}
					if err = w.WriteBody([]byte("\r\n")); err != nil {
						return err
					}
				}

				if errors.Is(rErr, io.EOF) {
					break
				}
				if rErr != nil {
					return rErr
				}
			}
			if err = w.WriteBody([]byte("0\r\n")); err != nil {
				return err
			}
			trailer := headers.NewHeaders()
			sum := sha256.Sum256(fullBody)
			trailer.Set("X-Content-SHA256", hex.EncodeToString(sum[:]))
			trailer.Set("X-Content-Length", fmt.Sprintf("%d", len(fullBody)))
			if err = w.WriteHeaders(trailer); err != nil {
				return err
			}
		}

	} else {
		status = response.StatusNotFound
		body = respond404()
	}

	if !chunked {
		h.Replace("Content-Length", strconv.Itoa(len(body)))
		if err := w.WriteResponse(status, h, body); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	s, err := server.Serve(port, handler)
	if err != nil {
		log.Fatalf("error starting server: %v", err)
	}
	defer s.Close()
	log.Printf("Server started on port: %d", port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}
