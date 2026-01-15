package response

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ShazimR/tcp-http-server/internal/headers"
)

type StatusCode int

const (
	StatusOK                  StatusCode = 200
	StatusBadRequest          StatusCode = 400
	StatusInternalServerError StatusCode = 500
)

var (
	ErrUnrecognizedStatusCode = fmt.Errorf("unrecognized status code")
)

type Response struct {
}

func GetDefaultHeaders(contentLen int) *headers.Headers {
	h := headers.NewHeaders()
	h.Set("Content-Length", strconv.Itoa(contentLen))
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/plain")

	return h
}

func WriteStatusLine(w io.Writer, statusCode StatusCode) error {
	statusLine := []byte{}
	switch statusCode {
	case StatusOK:
		statusLine = []byte("HTTP/1.1 200 OK\r\n")
	case StatusBadRequest:
		statusLine = []byte("HTTP/1.1 400 Bad Request\r\n")
	case StatusInternalServerError:
		statusLine = []byte("HTTP/1.1 500 Internal Server Error\r\n")
	default:
		return ErrUnrecognizedStatusCode
	}

	_, err := w.Write(statusLine)
	return err
}

func WriteHeaders(w io.Writer, h *headers.Headers) error {
	b := []byte{}

	h.ForEach(func(name, value string) {
		b = fmt.Appendf(b, "%s: %s\n", name, value)
	})
	b = fmt.Appendf(b, "\r\n")

	_, err := w.Write(b)
	return err
}
