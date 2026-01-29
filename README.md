[![CI](https://github.com/ShazimR/tcp-http-server/actions/workflows/go.yml/badge.svg)](
https://github.com/ShazimR/tcp-http-server/actions/workflows/go.yml
)


# tcp-http-server

A **from-scratch HTTP/1.1 server written in Go**, built directly on top of TCP sockets.  
This project intentionally avoids Go's `net/http` package to explore how HTTP actually works at the protocol level.

The focus is on **correctness, clarity, and learning**, not performance or production readiness.


## Table of Contents

* [Overview](#overview)
* [Features](#features)

  * [HTTP Core](#http-core)
  * [Responses](#responses)
  * [Router](#router)
  * [Static Files](#static-files)
  * [Tooling](#tooling)
* [Project Structure](#project-structure)
* [Running the Server](#running-the-server)

  * [Without Router (manual handling)](#without-router-manual-handling)
  * [With Router](#with-router)
* [Router Example](#router-example)

  * [Router Behavior](#router-behavior)
* [Example API Call](#example-api-call)
* [Static Router Tester](#static-router-tester)
* [Design Decisions](#design-decisions)
* [Lessons Learned](#lessons-learned)
* [Non-Goals](#non-goals)
* [Future Work](#future-work)
* [Why This Project](#why-this-project)
* [Author](#author)


## Overview

This project implements a minimal but functional HTTP/1.1 server, including:

- TCP connection handling
- HTTP request parsing (including chunked request bodies)
- HTTP response formatting
- Chunked transfer encoding
- Static file serving
- Method-based routing with path parameters
- Query parameter parsing
- Partial content support (`Range`, `206`, `416`)
- Graceful shutdown
- Unit testing, CI, and Docker support

Two example servers are provided:
- one **without a router** (manual request handling)
- one **with a custom router** implementation


## Features

### HTTP Core
- TCP-based server using `net.Listen` and `net.Conn`
- Manual parsing of:
  - request line
  - headers
  - body
  - query parameters
- Supports:
  - `Content-Length` bodies
  - `Transfer-Encoding: chunked` request bodies
- Proper CRLF handling
- Partial read/write handling
- Binary-safe parsing and responses
- Populates:
  - `req.RequestParams`

### Responses
- Status line + headers + body
- `Content-Length` responses
- `Transfer-Encoding: chunked`
- Optional trailers
- Streaming file responses
- Partial Content:
  - `Range` request parsing
  - `206 Partial Content`
  - `416 Range Not Satisfiable`
  - `Content-Range` and `Accept-Ranges`

### Router
- Method-based routing (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`, etc.)
- Static path matching
- Path parameters (e.g. `/api/users/:userid/posts/:postid`)
- Correct distinction between:
  - `404 Not Found`
  - `405 Method Not Allowed`
- Populates:
  - `req.PathParams`

### Static Files
- Serve HTML, CSS, JavaScript, favicon, and video
- Correct `Content-Type` handling
- Chunked file streaming for large assets
- Partial content support for media playback

### Tooling
- Unit tests for core components
- GitHub Actions workflow for build and tests
- Dockerfile and docker-compose support


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
│   ├── favicon.ico
│   └── video.mp4
│
├── Dockerfile
├── docker-compose.yml
└── README.md

```


## Running the Server

### Without Router (manual handling)

```bash
go run ./cmd/httpserver/main.go
````

Demonstrates:

* explicit routing logic
* chunked responses
* chunked request body parsing
* static file streaming
* partial content handling


### With Router

```bash
go run ./cmd/httpserver-router/main.go
```

Demonstrates:

* method-based routing
* path parameter parsing
* query parameter parsing
* cleaner separation of concerns


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

r.GET("/api/users/:userid/posts/:postid", echoParams)
...
```

### Router Behavior

* Routing matches **path only**
* Query parameters are parsed separately (request parser)
* Path parameters populate `req.PathParams`
* Query parameters populate `req.RequestParams` (request parser)
* Correct `404` vs `405` behavior


## Example API Call

```bash
curl -X POST "http://localhost:8080/api/echo?flag=true&test=a=b" \
  -H "Content-Type: application/json" \
  -d '{"msg":"hello","ts":123}'
```

Response:

```json
{
  "Method": "POST",
  "Path": "/api/echo",
  "PathParams": {},
  "QueryParams": {
    "flag": "true",
    "test": "a=b"
  },
  "Body": "msg: hello | ts: 123"
}
```


## Static Router Tester

A small static website is included and served at `/`:

* Buttons for `GET`, `POST`, `PUT`, `DELETE`, `PATCH`
* Separate query param inputs
* Path parameter testing
* Raw request preview
* No database
* Media streaming
* Intended for demonstration and presentation
  (debugging is done via `curl`)


## Docker

Build and run using Docker Compose:

```bash
docker compose up --build
```

Exposes the server on:

```
http://localhost:8080
```

The Docker image is built as a **static binary** (no shell, minimal attack surface).


Love this addition — a **Design Decisions** section really elevates this from “cool project” to “engineer who thinks deeply about systems.”

Below is a **clean, concise, portfolio-ready Design Decisions section** you can drop straight into your README. It explains *why* you did things the way you did, without turning into a blog post.


## Design Decisions

### Build on raw TCP - not `net/http`

The server is implemented directly on top of `net.Listen` and `net.Conn`, avoiding Go's `net/http` package entirely.
This forces explicit handling of framing, partial reads/writes, CRLF parsing, and connection lifecycles, providing a clear mental model of how HTTP actually operates over TCP.

### Incremental parsing with explicit state machines

Request parsing is implemented as a state machine (`request-line → headers → body → chunks → trailers`).
This allows the parser to:

* tolerate partial reads
* correctly handle streaming bodies
* support both `Content-Length` and chunked request bodies
* gracefully detect malformed input

This mirrors how production servers must parse untrusted network input.

### Separate routing from request parsing

Routing is intentionally decoupled from request parsing:

* The **router matches only on the path**
* Query parameters are parsed independently
* Path parameters are extracted during routing

This avoids conflating routing concerns with request decoding and makes router behavior predictable and testable.

### Path parameters as first-class citizens

Routes are represented as a tree of path segments, with explicit support for parameter nodes (e.g. `:userid`).
This enables:

* fast lookup
* deterministic matching
* clean population of `req.PathParams`
* correct handling of ambiguous routes

### Explicit distinction between `404` and `405`

The router tracks all handlers registered at a path.
If a path exists but the method does not, the server returns **`405 Method Not Allowed`** instead of `404`.

This mirrors real HTTP server behavior and avoids a common correctness bug in simple routers.

### Chunked encoding implemented end-to-end

Both **chunked responses** *and* **chunked request bodies** are supported.

Special care is taken to:

* write chunk sizes correctly
* terminate with `0\r\n`
* optionally handle trailers
* avoid treating binary data as text

This ensures compatibility with browsers and streaming clients.

### Binary safety everywhere

All I/O paths treat payloads as raw bytes:

* no implicit string conversions
* no newline assumptions
* safe for video, images, and arbitrary binary data

This was necessary for correct media streaming and chunked transfers.

### Partial content support driven by real browser behavior

`Range` request handling (`206`, `416`, `Content-Range`) was implemented after observing real browser requests for media playback.

The implementation:

* validates ranges
* clamps end offsets
* supports open-ended ranges (`bytes=0-`)
* correctly reports file size

This ensures browser video scrubbing works correctly.

### Minimal abstractions, not frameworks

Only a few core abstractions exist:

* `Request`
* `Writer`
* `Router`

Each abstraction exists to **prevent bugs**, not to hide complexity.
The goal is clarity and correctness rather than convenience.

### Docker image built as a static binary

The Docker image builds a statically linked binary (`CGO_ENABLED=0`) and runs it in a minimal container with no shell.

This:

* reduces attack surface
* mirrors real production container practices
* reinforces understanding of Go's build pipeline

### Tests focus on behavior, not implementation

Tests validate observable behavior:

* correct parsing
* correct routing
* correct error handling

They intentionally avoid comparing function pointers or internal state, which is brittle and misleading in Go.


## Lessons Learned

* HTTP is deceptively simple — complexity lives in edge cases.
* TCP provides no message boundaries; HTTP framing must be explicit.
* Partial reads and writes are the default, not exceptions.
* A successful `Write()` does not guarantee the full buffer was written.
* CRLF placement is mandatory — one missing byte breaks clients.
* Browsers are far less forgiving than `curl`.
* `Content-Length` and `Transfer-Encoding: chunked` are mutually exclusive.
* Chunked encoding is easy to get subtly wrong.
* Chunked *request* bodies must be parsed incrementally.
* Trailers must be declared before the body and written after the terminating chunk.
* Binary data must never be treated as text.
* Browsers implicitly issue `Range` requests for media.
* Partial content requires strict validation (`206`, `416`, `Content-Range`).
* Routing must distinguish “path does not exist” vs “method not allowed”.
* Path and query parameters must be parsed independently.
* Function identity cannot be tested — behavior must be observed.
* Correct `Content-Type` headers are critical for browser behavior.
* Graceful shutdown requires coordinating listeners and connections.
* Small abstractions drastically improve correctness.
* `net/http` hides a *lot* of complexity — understanding it is invaluable.


## Non-Goals

This project intentionally does **not** implement:

* HTTP/2 or HTTP/3
* TLS / HTTPS
* Persistent connections
* Middleware
* Full RFC compliance


## Future Work

* Persistent connections (`keep-alive`)
* Middleware support
* Router groups and path prefixes
* Request body streaming (avoid full buffering)
* Automatic `OPTIONS` + `Allow`
* Compression
* Improved protocol compliance
* TLS / HTTPS
* Fuzz and integration testing


## Why This Project

This project exists as a **learning exercise** to deeply understand HTTP and TCP behavior rather than to replace Go's standard libraries.

If you understand this code, you understand:

* why HTTP servers are structured the way they are
* what real servers must do to be correct
* how abstractions like `net/http` earn their complexity


## Author

[**Shazim Rahman**](https://github.com/ShazimR)

Built as an educational deep dive into HTTP, TCP, and systems programming in Go.
