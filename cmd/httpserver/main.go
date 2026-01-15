package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

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

func handler(w *response.Writer, req *request.Request) error {
	var status response.StatusCode
	var body []byte

	switch req.RequestLine.RequestTarget {
	case "/", "/shazim":
		status = response.StatusOK
		body = respond200()

	case "/yourproblem":
		status = response.StatusBadRequest
		body = respond400()

	case "/myproblem":
		status = response.StatusInternalServerError
		body = respond500()

	default:
		status = response.StatusNotFound
		body = respond404()
	}

	h := response.GetDefaultHeaders(len(body))

	err := w.WriteResponse(status, h, body)
	if err != nil {
		return err
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
