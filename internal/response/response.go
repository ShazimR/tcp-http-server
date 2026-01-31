package response

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ShazimR/tcp-http-server/internal/headers"
	"github.com/ShazimR/tcp-http-server/internal/request"
)

type StatusCode uint

const (
	StatusOK                      StatusCode = 200
	StatusCreated                 StatusCode = 201
	StatusPartialContent          StatusCode = 206
	StatusBadRequest              StatusCode = 400
	StatusNotFound                StatusCode = 404
	StatusMethodNotAllowed        StatusCode = 405
	StatusRangeNotSatisfiable     StatusCode = 416
	StatusInternalServerError     StatusCode = 500
	StatusNotImplemented          StatusCode = 501
	StatusHttpVersionNotSupported StatusCode = 505
)

var (
	ErrUnrecognizedStatusCode = fmt.Errorf("unrecognized status code")
	ErrFailedToWrite          = fmt.Errorf("failed to write")
	ErrRangeOutOfBounds       = fmt.Errorf("range start out of bounds")
	ErrRangeEndLtStart        = fmt.Errorf("range end < start")
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
	case StatusCreated:
		statusLine = []byte("HTTP/1.1 201 Created\r\n")
	case StatusPartialContent:
		statusLine = []byte("HTTP/1.1 206 Partial Content\r\n")
	case StatusBadRequest:
		statusLine = []byte("HTTP/1.1 400 Bad Request\r\n")
	case StatusNotFound:
		statusLine = []byte("HTTP/1.1 404 Not Found\r\n")
	case StatusMethodNotAllowed:
		statusLine = []byte("HTTP/1.1 405 Method Not Allowed\r\n")
	case StatusRangeNotSatisfiable:
		statusLine = []byte("HTTP/1.1 416 Range Not Satisfiable\r\n")
	case StatusInternalServerError:
		statusLine = []byte("HTTP/1.1 500 Internal Server Error\r\n")
	case StatusNotImplemented:
		statusLine = []byte("HTTP/1.1 501 Not Implemented\r\n")
	case StatusHttpVersionNotSupported:
		statusLine = []byte("HTTP/1.1 505 Http Version Not Supported\r\n")
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

func (w *Writer) WritePartialContentResponse(f io.ReadSeeker, contentSize int, contentType string, req *request.Request) error {
	h := GetDefaultHeaders(0)
	h.Replace("Content-Type", contentType)
	h.Set("Accept-Ranges", "bytes")

	if rangeStr, ok := req.Headers.Get("Range"); ok {
		start, end, endProvided, ok := parseRange(rangeStr)
		if !ok {
			body := []byte("invalid range")
			h.Replace("Content-Type", "text/plain")
			h.Replace("Content-Length", strconv.Itoa(len(body)))
			return w.WriteResponse(StatusBadRequest, h, body)
		}

		body, usedEnd, err := loadRange(f, contentSize, start, end, endProvided)
		if errors.Is(err, ErrRangeEndLtStart) || errors.Is(err, ErrRangeOutOfBounds) {
			body = []byte("invalid range provided")
			h.Replace("Content-Type", "text/plain")
			h.Replace("Content-Length", strconv.Itoa(len(body)))
			h.Set("Content-Range", fmt.Sprintf("bytes */%d", contentSize))
			return w.WriteResponse(StatusRangeNotSatisfiable, h, body)
		}
		if err != nil {
			body = []byte("error loading range")
			h.Replace("Content-Type", "text/plain")
			h.Replace("Content-Length", strconv.Itoa(len(body)))
			return w.WriteResponse(StatusInternalServerError, h, body)
		}

		h.Replace("Content-Length", strconv.Itoa(len(body)))
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, usedEnd, contentSize))
		return w.WriteResponse(StatusPartialContent, h, body)

	} else {
		body, err := io.ReadAll(f)
		if err != nil {
			body = []byte("error loading content")
			h.Replace("Content-Type", "text/plain")
			h.Replace("Content-Length", strconv.Itoa(len(body)))
			return w.WriteResponse(StatusInternalServerError, h, body)
		}

		h.Replace("Content-Length", strconv.Itoa(len(body)))
		return w.WriteResponse(StatusOK, h, body)
	}
}

func GetDefaultHeaders(contentLen int) *headers.Headers {
	h := headers.NewHeaders()
	h.Set("Content-Length", strconv.Itoa(contentLen))
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/html")

	return h
}

func parseRange(s string) (start int, end int, endprovided bool, ok bool) {
	prefix := "bytes="
	if !strings.HasPrefix(s, prefix) {
		return 0, 0, false, false
	}

	rangeStr := strings.TrimPrefix(s, prefix)
	parts := strings.SplitN(rangeStr, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, false
	}

	if parts[0] == "" {
		return 0, 0, false, false
	}

	st, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false, false
	}

	if parts[1] == "" {
		return st, 0, false, true
	}

	en, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false, false
	}

	return st, en, true, true
}

func loadRange(f io.ReadSeeker, contentSize int, start int, end int, endProvided bool) (body []byte, usedEnd int, err error) {
	if contentSize <= 0 {
		return nil, -1, ErrRangeOutOfBounds
	}

	if start >= contentSize {
		return nil, 0, ErrRangeOutOfBounds
	}

	if !endProvided {
		end = contentSize - 1
	} else {
		if end < start {
			return nil, 0, ErrRangeEndLtStart
		}
		if end >= contentSize {
			end = contentSize - 1 // clamp
		}
	}

	n := (end - start) + 1

	if _, err := f.Seek(int64(start), io.SeekStart); err != nil {
		return nil, 0, err
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, 0, err
	}

	return buf, end, nil
}
