package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/ShazimR/tcp-http-server/internal/router"
	"github.com/ShazimR/tcp-http-server/internal/server"
)

const port = 8080
const maxChunkSize = 1024

type TestResponse struct {
	Message   string `json:"msg"`
	Timestamp uint64 `json:"ts"`
}

func serveStatic(filename string, contentType string, w *response.Writer) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", contentType)

	body, err := os.ReadFile(filename)
	if err != nil {
		status = response.StatusInternalServerError
		body = []byte("error loading file")
		h.Replace("Content-Type", "text/plain")
	}

	h.Replace("Content-Length", strconv.Itoa(len(body)))
	err = w.WriteResponse(status, h, body)
	return err
}

func serveChunked(filename string, contentType string, w *response.Writer) error {
	h := response.GetDefaultHeaders(0)
	f, err := os.Open(filename)
	if err != nil {
		body := []byte("error opening file")
		w.WriteResponse(response.StatusInternalServerError, h, body)
		return err
	}
	defer f.Close()

	status := response.StatusOK
	h.Replace("Content-Type", contentType)
	h.Delete("Content-Length")
	h.Set("Transfer-Encoding", "chunked")

	if err := w.WriteStatusLine(status); err != nil {
		return err
	}
	if err := w.WriteHeaders(h); err != nil {
		return err
	}

	data := [maxChunkSize]byte{}
	for {
		n, rErr := f.Read(data[:])

		if n > 0 {
			if err := w.WriteChunk(data[:n]); err != nil {
				return err
			}
		}

		if errors.Is(rErr, io.EOF) {
			break
		}
		if rErr != nil {
			return rErr
		}
	}

	if err := w.WriteChunkEnd(false); err != nil {
		return err
	}

	return nil
}

// Static handlers
func serveIndex(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/index.html", "text/html", w)
}

func serveFavicon(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/favicon.ico", "image/x-icon", w)
}

func serveStyles(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/styles.css", "text/css", w)
}

func serveApp(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/app.js", "text/javascript", w)
}

func serveVideo(w *response.Writer, req *request.Request) error {
	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	bodyStr := parseDemoBody(req)

	fmt.Printf("Method:      %s\n", method)
	fmt.Printf("Path:        %s\n", path)
	fmt.Printf("QueryParams: %s\n", req.RequestParams)
	fmt.Printf("Body:        %s\n", bodyStr)
	fmt.Printf("Raw:         %s\n\n", req.Body)

	return serveStatic("./static/one-last-breath.mp4", "video/mp4", w)
}

func serveVideoChunked(w *response.Writer, req *request.Request) error {
	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	bodyStr := parseDemoBody(req)

	fmt.Printf("Method:      %s\n", method)
	fmt.Printf("Path:        %s\n", path)
	fmt.Printf("QueryParams: %s\n", req.RequestParams)
	fmt.Printf("Body:        %s\n", bodyStr)
	fmt.Printf("Raw:         %s\n\n", req.Body)

	return serveChunked("./static/one-last-breath.mp4", "video/mp4", w)
}

// Shared helper for printing/parsing the demo JSON body
func parseDemoBody(req *request.Request) string {
	var reqBody TestResponse
	if err := json.Unmarshal(req.Body, &reqBody); err != nil {
		return "failed to parse body or body was empty"
	}
	return fmt.Sprintf("msg: %s | ts: %d", reqBody.Message, reqBody.Timestamp)
}

func echo(w *response.Writer, req *request.Request) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", "application/json")

	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	bodyStr := parseDemoBody(req)

	fmt.Printf("Method:      %s\n", method)
	fmt.Printf("Path:        %s\n", path)
	fmt.Printf("QueryParams: %s\n", req.RequestParams)
	fmt.Printf("Body:        %s\n", bodyStr)
	fmt.Printf("Raw:         %s\n\n", req.Body)

	resp := map[string]any{
		"Method":      method,
		"Path":        path,
		"QueryParams": req.RequestParams,
		"Body":        bodyStr,
	}

	resBody, err := json.Marshal(resp)
	if err != nil {
		status = response.StatusInternalServerError
		resBody = []byte("failed to jsonify output")
		h.Replace("Content-Type", "text/plain")
	}

	h.Replace("Content-Length", strconv.Itoa(len(resBody)))
	return w.WriteResponse(status, h, resBody)
}

// Echo that also includes path params in the response
func echoParams(w *response.Writer, req *request.Request) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", "application/json")

	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	bodyStr := parseDemoBody(req)

	fmt.Printf("Method:      %s\n", method)
	fmt.Printf("Path:        %s\n", path)
	fmt.Printf("PathParams:  %s\n", req.PathParams)
	fmt.Printf("QueryParams: %s\n", req.RequestParams)
	fmt.Printf("Body:        %s\n", bodyStr)
	fmt.Printf("RawBody:     %s\n\n", req.Body)

	resp := map[string]any{
		"Method":      method,
		"Path":        path,
		"PathParams":  req.PathParams,
		"QueryParams": req.RequestParams,
		"Body":        bodyStr,
	}

	resBody, err := json.Marshal(resp)
	if err != nil {
		status = response.StatusInternalServerError
		resBody = []byte("failed to jsonify output")
		h.Replace("Content-Type", "text/plain")
	}

	h.Replace("Content-Length", strconv.Itoa(len(resBody)))
	return w.WriteResponse(status, h, resBody)
}

func main() {
	// Static assets
	r := router.NewRouter()
	r.GET("/", serveIndex)
	r.GET("/index.html", serveIndex)
	r.GET("/favicon.ico", serveFavicon)
	r.GET("/styles.css", serveStyles)
	r.GET("/app.js", serveApp)
	r.GET("/video", serveVideo)
	r.GET("/video-chunked", serveVideoChunked)

	// Base echo (no path params needed)
	// Example target /api/echo?flag=true&test=a=b
	r.GET("/api/echo", echo)
	r.POST("/api/echo", echo)
	r.PUT("/api/echo", echo)
	r.DELETE("/api/echo", echo)
	r.PATCH("/api/echo", echo)

	// Path param-aware echo
	// Example target /api/echo/123?flag=true&test=a=b
	r.GET("/api/echo/:id", echoParams)
	r.POST("/api/echo/:id", echoParams)
	r.PUT("/api/echo/:id", echoParams)
	r.DELETE("/api/echo/:id", echoParams)
	r.PATCH("/api/echo/:id", echoParams)

	// Multi path param example
	// Example target /api/users/7/posts/99?view=full
	r.GET("/api/users/:userid/posts/:postid", echoParams)
	r.POST("/api/users/:userid/posts/:postid", echoParams)

	// Setup and run server
	s, err := server.Serve(port, nil, r)
	if err != nil {
		log.Fatalf("error starting server: %v", err)
	}
	defer s.Close()
	log.Printf("Server started on port: %d", port)

	// Stop server gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}
