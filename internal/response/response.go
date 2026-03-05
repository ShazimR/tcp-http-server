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
	StatusUnauthorized            StatusCode = 401
	StatusNotFound                StatusCode = 404
	StatusMethodNotAllowed        StatusCode = 405
	StatusRequestTimeout          StatusCode = 408
	StatusRangeNotSatisfiable     StatusCode = 416
	StatusInternalServerError     StatusCode = 500
	StatusNotImplemented          StatusCode = 501
	StatusHttpVersionNotSupported StatusCode = 505
)

var statusText = map[StatusCode]string{
	StatusOK:                      "OK",
	StatusCreated:                 "Created",
	StatusPartialContent:          "Partial Content",
	StatusBadRequest:              "Bad Request",
	StatusUnauthorized:            "Unauthorized",
	StatusNotFound:                "Not Found",
	StatusMethodNotAllowed:        "Method Not Allowed",
	StatusRequestTimeout:          "Request Timeout",
	StatusRangeNotSatisfiable:     "Range Not Satisfiable",
	StatusInternalServerError:     "Internal Server Error",
	StatusNotImplemented:          "Not Implemented",
	StatusHttpVersionNotSupported: "HTTP Version Not Supported",
}

func (s StatusCode) String() string {
	if text, ok := statusText[s]; ok {
		return text
	}
	return ""
}

var (
	ErrUnrecognizedStatusCode = fmt.Errorf("unrecognized status code")
	ErrFailedToWrite          = fmt.Errorf("failed to write")
	ErrRangeOutOfBounds       = fmt.Errorf("range start out of bounds")
	ErrRangeEndLtStart        = fmt.Errorf("range end < start")
)

type Handler func(w *Writer, req *request.Request) error

type logger struct {
	status     StatusCode
	statusLine []byte
	header     []byte
	body       []byte
}

type Writer struct {
	writer      io.Writer
	logs        *logger
	shouldClose bool
	httpVersion string
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer:      w,
		logs:        &logger{},
		shouldClose: false,
		httpVersion: "1.1",
	}
}

func (w *Writer) ResetLogs() {
	w.logs = &logger{}
}

func (w *Writer) GetLogs() (statusLine string, headers string, body []byte) {
	return string(w.logs.statusLine), string(w.logs.header), w.logs.body
}

func (w *Writer) GetStatus() (status StatusCode) {
	return w.logs.status
}

func (w *Writer) SetHttpVersion(httpVersion string) {
	w.httpVersion = httpVersion
}

func (w *Writer) ForceCloseConnection() {
	w.shouldClose = true
}

func (w *Writer) WriteStatusLine(statusCode StatusCode) error {
	text := statusCode.String()
	statusLine := fmt.Appendf(nil, "HTTP/%s %d %s\r\n", w.httpVersion, statusCode, text)
	if text == "" {
		return ErrUnrecognizedStatusCode
	}
	w.logs.status = statusCode
	w.logs.statusLine = statusLine

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
	if w.shouldClose {
		h.Replace("Connection", "close")
	}

	b := []byte{}
	h.ForEach(func(name, value string) {
		b = fmt.Appendf(b, "%s: %s\r\n", name, value)
	})
	b = fmt.Appendf(b, "\r\n")
	w.logs.header = b

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
	w.logs.body = append(w.logs.body, p...)
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
	h.Set("Connection", "keep-alive")
	h.Set("Content-Type", "text/html")

	return h
}

func SetCookie(h *headers.Headers, key string, value string, args ...string) {
	var cookieStr strings.Builder
	cookieStr.WriteString(key + "=" + value)
	for _, arg := range args {
		fmt.Fprintf(&cookieStr, "; %s", arg)
	}
	h.SetGrouped("Set-Cookie", cookieStr.String())
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
