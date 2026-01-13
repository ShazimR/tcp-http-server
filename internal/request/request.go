package request

import (
	"bytes"
	"fmt"
	"io"

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
	RequestLine RequestLine
	Headers     *headers.Headers
	state       parserState
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
	StateDone
	StateError
)

func newRequest() *Request {
	return &Request{
		state:   StateInit,
		Headers: headers.NewHeaders(),
	}
}

func (r *Request) done() bool {
	return r.state == StateDone || r.state == StateError
}

func (r *Request) parse(data []byte) (int, error) {
	read := 0

outer:
	for {
		currentData := data[read:]

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

	return request, nil
}
