package headers

import (
	"bytes"
	"fmt"
	"strings"
)

var (
	sepCRLF = []byte("\r\n")
	sepSP   = []byte(" ")
)

var (
	ErrMalformedHeader     = fmt.Errorf("malformed header")
	ErrMalformedFieldLine  = fmt.Errorf("malformed field line")
	ErrMalformedHeaderName = fmt.Errorf("malformed header name")
)

func isToken(str []byte) bool {
	// _ :;.,\/"'?!(){}[]@<>=-+*#$&`|~^%
	for _, ch := range str {
		found := false
		if ch >= 'A' && ch <= 'Z' ||
			ch >= 'a' && ch <= 'z' ||
			ch >= '0' && ch <= 'z' {
			found = true
		}
		switch ch {
		case '_', ':', ';', '.', ',', '\\', '/', '"', '\'', '?', '!', '(', ')', '{', '}', '[', ']', '@', '<', '>', '=', '-', '+', '*', '#', '$', '&', '`', '|', '~', '^', '%':
			found = true
		}

		if !found {
			return false
		}
	}

	return true
}

func parseHeader(fieldLine []byte) (string, string, error) {
	parts := bytes.SplitN(fieldLine, []byte(":"), 2)
	if len(parts) != 2 {
		return "", "", ErrMalformedHeader
	}

	name := parts[0]
	value := bytes.TrimSpace(parts[1])
	if bytes.HasSuffix(name, sepSP) || bytes.HasPrefix(name, sepSP) {
		return "", "", ErrMalformedFieldLine
	}

	return string(name), string(value), nil
}

type Headers struct {
	headers map[string]string
}

func NewHeaders() *Headers {
	return &Headers{
		headers: map[string]string{},
	}
}

func (h *Headers) Get(name string) (string, bool) {
	str, ok := h.headers[strings.ToLower(name)]
	return str, ok
}

func (h *Headers) Replace(name string, value string) {
	name = strings.ToLower(name)
	h.headers[name] = value
}

func (h *Headers) Set(name string, value string) {
	name = strings.ToLower(name)

	if v, ok := h.headers[name]; ok {
		h.headers[name] = fmt.Sprintf("%s,%s", v, value)
	} else {
		h.headers[name] = value
	}
}

func (h *Headers) ForEach(cb func(name, value string)) {
	for n, v := range h.headers {
		cb(n, v)
	}
}

func (h *Headers) Parse(data []byte) (int, bool, error) {
	read := 0
	done := false

	for {
		idx := bytes.Index(data[read:], sepCRLF)
		if idx == -1 {
			break
		}

		// Empty Header
		if idx == 0 {
			done = true
			read += len(sepCRLF)
			break
		}

		name, value, err := parseHeader(data[read : read+idx])
		if err != nil {
			return 0, false, err
		}

		if !isToken([]byte(name)) {
			return 0, false, ErrMalformedHeaderName
		}

		read += idx + len(sepCRLF)

		h.Set(name, value)
	}

	return read, done, nil
}
