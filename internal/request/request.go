package request

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ShazimR/tcp-http-server/internal/headers"
)

var (
	sepCRLF = []byte("\r\n")
	sepSP   = []byte(" ")
)

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

type Request struct {
	RequestLine   RequestLine
	Headers       *headers.Headers
	Body          []byte
	Trailer       *headers.Headers
	RequestParams map[string]string
	PathParams    map[string]string
	state         parserState
}

var (
	ErrMalformedRequestLine = fmt.Errorf("malformed request-line")
	ErrUnsupportedVersion   = fmt.Errorf("unsupported http version")
	ErrReqInErrState        = fmt.Errorf("request in error state")
)

type parserState int

const (
	StateInit parserState = iota
	StateHeaders
	StateBody
	StateChunkLength
	StateChunkData
	StateTrailer
	StateDone
	StateError
)

func getInt(headers *headers.Headers, name string, defaultValue int) int {
	valueStr, exists := headers.Get(name)
	if !exists {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

func newRequest() *Request {
	return &Request{
		state:         StateInit,
		Headers:       headers.NewHeaders(),
		Body:          []byte{},
		Trailer:       headers.NewHeaders(),
		RequestParams: make(map[string]string),
		PathParams:    make(map[string]string),
	}
}

func (r *Request) done() bool {
	return r.state == StateDone || r.state == StateError
}

func (r *Request) getBodyState() parserState {
	state := StateDone
	length := getInt(r.Headers, "content-length", 0)
	chunked, ok := r.Headers.Get("transfer-encoding")

	if ok && chunked == "chunked" {
		state = StateChunkLength
	} else if length > 0 {
		state = StateBody
	}

	return state
}

func (r *Request) parse(data []byte) (int, error) {
	read := 0

outer:
	for {
		currentData := data[read:]
		if len(currentData) == 0 {
			break outer
		}

		switch r.state {
		case StateError:
			return 0, ErrReqInErrState

		case StateInit:
			rl, n, err := parseRequestLine(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}

			if n == 0 {
				break outer
			}

			r.RequestLine = *rl
			read += n
			r.state = StateHeaders

		case StateHeaders:
			n, done, err := r.Headers.Parse(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}

			if n == 0 {
				break outer
			}

			read += n

			if done {
				r.state = r.getBodyState()
			}

		case StateBody:
			length := getInt(r.Headers, "content-length", 0)

			remaining := min(length-len(r.Body), len(currentData))
			r.Body = append(r.Body, currentData[:remaining]...)
			read += remaining

			if len(r.Body) == length {
				r.state = StateDone
			}

		case StateChunkLength:
			n, l, err := parseChunkLength(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}
			if n == 0 {
				break outer
			}

			read += n
			if l == 0 {
				if _, ok := r.Headers.Get("Trailer"); ok {
					r.state = StateTrailer
				} else {
					r.state = StateDone
				}

			} else {
				r.state = StateChunkData
			}

		case StateChunkData:
			n := parseChunkData(currentData, r)
			if n == 0 {
				break outer
			}

			read += n
			r.state = StateChunkLength

		case StateTrailer:
			n, done, err := r.Trailer.Parse(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}

			if n == 0 {
				break outer
			}

			read += n

			if done {
				r.state = StateDone
			}

		case StateDone:
			break outer

		default:
			panic("entered default parser state")
		}
	}

	return read, nil
}

func parseRequestLine(b []byte) (*RequestLine, int, error) {
	idx := bytes.Index(b, sepCRLF)
	if idx == -1 {
		return nil, 0, nil // not enough data yet
	}

	startLine := b[:idx]
	read := idx + len(sepCRLF)

	parts := bytes.Split(startLine, sepSP)
	if len(parts) != 3 {
		return nil, 0, ErrMalformedRequestLine
	}

	httpParts := bytes.Split(parts[2], []byte("/"))
	if len(httpParts) != 2 || string(httpParts[0]) != "HTTP" {
		return nil, 0, ErrMalformedRequestLine
	}
	if string(httpParts[1]) != "1.1" {
		return nil, 0, ErrUnsupportedVersion
	}

	requestLine := &RequestLine{
		Method:        string(parts[0]),
		RequestTarget: string(parts[1]),
		HttpVersion:   string(httpParts[1]),
	}

	return requestLine, read, nil
}

func parseChunkLength(b []byte) (int, int, error) {
	idx := bytes.Index(b, sepCRLF)
	if idx == -1 {
		return 0, -1, nil // not enough data yet
	}

	lenHexStr := b[:idx]
	read := idx + len(sepCRLF)
	length, err := strconv.ParseUint(string(lenHexStr), 16, 64)
	if err != nil {
		return 0, -1, err
	}

	return read, int(length), nil
}

func parseChunkData(b []byte, r *Request) int {
	idx := bytes.Index(b, sepCRLF)
	if idx == -1 {
		return 0 // not enough data yet
	}

	data := b[:idx]
	read := idx + len(sepCRLF)
	r.Body = append(r.Body, data...)

	return read
}

func parseRequestParameters(r *Request) error {
	target := r.RequestLine.RequestTarget

	if i := strings.IndexByte(target, '?'); i != -1 && i < len(target)-1 {
		queryStr := target[i+1:]
		queries := strings.Split(queryStr, "&")

		for _, query := range queries {
			if k, v, hasEq := strings.Cut(query, "="); hasEq {
				r.RequestParams[k] = v

			} else if k != "" {
				r.RequestParams[k] = ""

			} else {
				return ErrMalformedRequestLine
			}
		}

		r.RequestLine.RequestTarget = target[:i]
	}

	return nil
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	request := newRequest()

	// NOTE: buffer could get overrun
	buf := make([]byte, 1024)
	bufLen := 0
	for !request.done() {
		n, err := reader.Read(buf[bufLen:])
		// TODO: what should we do here?
		if err != nil {
			return nil, err
		}

		bufLen += n
		readN, err := request.parse(buf[:bufLen])
		if err != nil {
			return nil, err
		}

		copy(buf, buf[readN:bufLen])
		bufLen -= readN
	}

	if err := parseRequestParameters(request); err != nil {
		return nil, err
	}

	return request, nil
}
