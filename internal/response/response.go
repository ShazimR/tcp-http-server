package response

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
)

type StatusCode int

const (
	StatusOK                  StatusCode = 200
	StatusBadRequest          StatusCode = 400
	StatusNotFound            StatusCode = 404
	StatusMethodNotAllowed    StatusCode = 405
	StatusInternalServerError StatusCode = 500
)

var (
	ErrUnrecognizedStatusCode = fmt.Errorf("unrecognized status code")
	ErrFailedToWrite          = fmt.Errorf("failed to write")
)

type Handler func(w *Writer, req *request.Request) error

type Writer struct {
	writer io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: w}
}

func (w *Writer) WriteStatusLine(statusCode StatusCode) error {
	statusLine := []byte{}
	switch statusCode {
	case StatusOK:
		statusLine = []byte("HTTP/1.1 200 OK\r\n")
	case StatusBadRequest:
		statusLine = []byte("HTTP/1.1 400 Bad Request\r\n")
	case StatusNotFound:
		statusLine = []byte("HTTP/1.1 404 Not Found\r\n")
	case StatusMethodNotAllowed:
		statusLine = []byte("HTTP/1.1 405 Method Not Allowed\r\n")
	case StatusInternalServerError:
		statusLine = []byte("HTTP/1.1 500 Internal Server Error\r\n")
	default:
		return ErrUnrecognizedStatusCode
	}

	writeN := 0
	for writeN < len(statusLine) {
		n, err := w.writer.Write(statusLine[writeN:])
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFailedToWrite, err)
		}
		if n == 0 {
			return fmt.Errorf("%w", ErrFailedToWrite)
		}
		writeN += n
	}

	return nil
}

func (w *Writer) WriteHeaders(h *headers.Headers) error {
	b := []byte{}

	h.ForEach(func(name, value string) {
		b = fmt.Appendf(b, "%s: %s\r\n", name, value)
	})
	b = fmt.Appendf(b, "\r\n")

	writeN := 0
	for writeN < len(b) {
		n, err := w.writer.Write(b[writeN:])
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFailedToWrite, err)
		}
		if n == 0 {
			return fmt.Errorf("%w", ErrFailedToWrite)
		}
		writeN += n
	}

	return nil
}

func (w *Writer) WriteBody(p []byte) error {
	writeN := 0
	for writeN < len(p) {
		n, err := w.writer.Write(p[writeN:])
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFailedToWrite, err)
		}
		if n == 0 {
			return fmt.Errorf("%w", ErrFailedToWrite)
		}
		writeN += n
	}

	return nil
}

func (w *Writer) WriteChunk(p []byte) error {
	if err := w.WriteBody(fmt.Appendf(nil, "%x\r\n", len(p))); err != nil {
		return err
	}
	if err := w.WriteBody(p); err != nil {
		return err
	}
	if err := w.WriteBody([]byte("\r\n")); err != nil {
		return err
	}

	return nil
}

func (w *Writer) WriteChunkEnd(hasTrailers bool) error {
	b := []byte{}
	if hasTrailers {
		b = []byte("0\r\n")
	} else {
		b = []byte("0\r\n\r\n")
	}

	if err := w.WriteBody(b); err != nil {
		return err
	}
	return nil
}

func (w *Writer) WriteResponse(statusCode StatusCode, header *headers.Headers, body []byte) error {
	if err := w.WriteStatusLine(statusCode); err != nil {
		return err
	}
	if err := w.WriteHeaders(header); err != nil {
		return err
	}
	if err := w.WriteBody(body); err != nil {
		return err
	}

	return nil
}

func GetDefaultHeaders(contentLen int) *headers.Headers {
	h := headers.NewHeaders()
	h.Set("Content-Length", strconv.Itoa(contentLen))
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/html")

	return h
}
