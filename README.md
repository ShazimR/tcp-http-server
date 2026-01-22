# tcp-http-server

A **from-scratch HTTP/1.1 server written in Go**, built directly on top of TCP sockets.  
This project intentionally avoids Go’s `net/http` package to explore how HTTP actually works at the protocol level.

The focus is on **correctness, clarity, and learning**, not performance or production readiness.

---

## Overview

This project implements a minimal but functional HTTP/1.1 server, including:

- TCP connection handling
- HTTP request parsing
- HTTP response formatting
- Chunked transfer encoding
- Static file serving
- Method-based routing
- Path parameter parsing
- Graceful shutdown
- Unit testing and CI

Two example servers are provided:
- one **without a router** (manual request handling)
- one **with a custom router** implementation

---

## Features

### HTTP Core
- TCP-based server using `net.Listen` and `net.Conn`
- Manual parsing of:
  - request line
  - headers
  - body
- Proper CRLF handling
- Partial read/write handling
- Binary-safe responses

### Responses
- Status line + headers + body
- `Content-Length` responses
- `Transfer-Encoding: chunked`
- Optional trailers
- Streaming file responses

### Router
- Method-based routing (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`, etc.)
- Exact path matching
- Path parameter parsing and population (`req.Params`)
- Correct distinction between:
  - `404 Not Found`
  - `405 Method Not Allowed`

### Static Files
- Serve HTML, CSS, JavaScript, favicon, and video
- Correct `Content-Type` handling
- Chunked file streaming for large assets

### Tooling
- Unit tests for core components
- GitHub Actions workflow for build and tests

---

## Project Structure

```
.
├── cmd/
│   ├── httpserver/                # Server without router (manual handler)
│   │   └── main.go
│   └── httpserver-router/         # Server using custom router
│       └── main.go
│
├── internal/
│   ├── headers/                   # HTTP headers abstraction
│   ├── request/                   # HTTP request parsing
│   ├── response/                  # HTTP response writer
│   ├── router/                    # Method + path router
│   └── server/                    # TCP server loop
│
├── static/                        # Static website for testing
│   ├── index.html
│   ├── styles.css
│   ├── app.js
│   └── favicon.ico
│
└── README.md

```

---

## Running the Server

### Without Router (manual handling)

```bash
go run ./cmd/httpserver/main.go
````

This version demonstrates:

* manual request inspection
* chunked responses
* static file streaming
* explicit routing logic

---

### With Router

```bash
go run ./cmd/httpserver-router/main.go
```

This version demonstrates:

* method-based routing
* path parameter parsing
* cleaner separation of concerns

---

## Router Example

```go
r := router.NewRouter()

r.GET("/", serveIndex)
r.GET("/index.html", serveIndex)
r.GET("/favicon.ico", serveFavicon)
r.GET("/styles.css", serveStyles)
r.GET("/app.js", serveApp)

r.GET("/api/echo", echo)
r.POST("/api/echo", echo)

r.GET("/api/echo?&id", echoParams)
r.GET("/api/echo?&id&flag", echoParams)
...
```

### Router Behavior

* Exact path matching
* Path parameters are parsed into `request.Params`
* Path parameters participate in routing
* Correct `404` vs `405` behavior

---

## Example API Call

```bash
curl -X POST http://localhost:8080/api/echo \
  -H "Content-Type: application/json" \
  -d '{"msg":"hello","ts":123}'
```

Response:

```json
{
  "Method": "POST",
  "Path": "/api/echo",
  "Params": "",
  "Body": "msg: hello | ts: 123"
}
```

---

## Static Router Tester

A small static website is included and served at `/`:

* Buttons for `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `OPTIONS`
* Live output display
* No database
* Useful for visually testing routing and responses

---

## Lessons Learned

- HTTP is deceptively simple — most complexity lives in edge cases, not the happy path.
- TCP provides no message boundaries; HTTP framing must be implemented explicitly.
- Partial reads and partial writes are the default, not the exception.
- A successful `Write()` does not guarantee the full buffer was written.
- Correct CRLF placement is mandatory; being off by one byte breaks clients.
- Browsers are far less forgiving than tools like `curl`.
- `Content-Length` and `Transfer-Encoding: chunked` are mutually exclusive.
- Chunked encoding is easy to implement incorrectly while still appearing to work.
- Trailers must be declared *before* the body and written *after* the terminating chunk.
- Binary data must never be treated as text — especially when streaming files.
- Request parsing must tolerate incomplete data and malformed input gracefully.
- Query string parsing has many edge cases (`=`, empty values, duplicate keys, encoding).
- URL decoding is required for correctness but easy to overlook.
- Correctly returning `405 Method Not Allowed` requires knowledge of *all* registered routes.
- Routing logic must distinguish between “path does not exist” and “method not allowed”.
- Function identity is not testable in Go — behavior must be tested instead.
- Handlers should be tested by observing effects, not by comparing references.
- Content-Type headers are critical for browser behavior (CSS and JS silently fail otherwise).
- Browsers may issue additional implicit requests (e.g. `favicon.ico`, `Range` headers).
- Graceful shutdown requires coordinating listener closure and active connections.
- Small abstractions (Writer, Router) drastically improve correctness and clarity.
- Go’s `net/http` hides a large amount of complexity — understanding what it hides is invaluable.
- Writing a server from scratch builds intuition that transfers directly to production systems.

---

## Non-Goals

This project intentionally does **not** implement:

* HTTP/2 or HTTP/3
* TLS / HTTPS
* Persistent connections
* Middleware
* Full RFC compliance

---

## Future Work

- Persistent connections (`Connection: keep-alive`)
- Middleware support (logging, recovery, authentication)
- Path prefix routing (router groups)
- Request body streaming (avoid full buffering)
- Automatic `OPTIONS` handling and `Allow` headers
- Content negotiation and compression
- Improved protocol compliance and robustness
- TLS / HTTPS support
- Fuzz and integration testing

---

## Why This Project

This project exists as a **learning exercise** to deeply understand HTTP and TCP behavior rather than to replace Go’s standard libraries.

If you understand this code, you understand:

* why HTTP servers are structured the way they are
* what real servers must do to be correct
* how abstractions like `net/http` earn their complexity

---

## Author

[<ins>**Shazim Rahman**</ins>](https://github.com/ShazimR)

Built as an educational deep dive into HTTP, TCP, and systems programming in Go.
