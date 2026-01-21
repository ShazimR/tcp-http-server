package main

import (
	"encoding/json"
	"fmt"
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

type TestResponse struct {
	Message   string `json:"msg"`
	Timestamp uint64 `json:"ts"`
}

// Helper functions
func load(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func serveStatic(filename string, contentType string, w *response.Writer) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", contentType)
	body, err := load(filename)
	if err != nil {
		status = response.StatusInternalServerError
		body = []byte("error loading file")
		h.Replace("Content-Type", "text/plain")
	}
	h.Replace("Content-Length", strconv.Itoa(len(body)))

	err = w.WriteResponse(status, h, body)
	return err
}

// Example route handlers
func serveIndex(w *response.Writer, req *request.Request) error {
	err := serveStatic("./static/index.html", "text/html", w)
	return err
}

func serveFavicon(w *response.Writer, req *request.Request) error {
	err := serveStatic("./static/favicon.ico", "image/x-icon", w)
	return err
}

func serveStyles(w *response.Writer, req *request.Request) error {
	err := serveStatic("./static/styles.css", "text/css", w)
	return err
}

func serveApp(w *response.Writer, req *request.Request) error {
	err := serveStatic("./static/app.js", "text/javascript", w)
	return err
}

func echo(w *response.Writer, req *request.Request) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", "application/json")

	var reqBody TestResponse
	reqBodyStr := ""
	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	err := json.Unmarshal(req.Body, &reqBody)
	if err != nil {
		reqBodyStr = "failed to parse body or body was empty"
	} else {
		reqBodyStr = fmt.Sprintf("msg: %s | ts: %d", reqBody.Message, reqBody.Timestamp)
	}

	fmt.Printf("Method:  %s\n", method)
	fmt.Printf("Path:    %s\n", path)
	fmt.Printf("Body:    %s\n", reqBodyStr)
	fmt.Printf("RawBody: %s\n\n", req.Body)

	body := map[string]string{
		"Method": method,
		"Path":   path,
		"Body":   reqBodyStr,
	}

	resBody, err := json.Marshal(body)
	if err != nil {
		status = response.StatusInternalServerError
		resBody = []byte("failed to jsonify output")
		h.Replace("Content-Type", "text/plain")
	}

	h.Replace("Content-Length", strconv.Itoa(len(resBody)))
	err = w.WriteResponse(status, h, resBody)
	return err
}

func echoParams(w *response.Writer, req *request.Request) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", "application/json")

	var reqBody TestResponse
	reqBodyStr := ""
	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	err := json.Unmarshal(req.Body, &reqBody)
	if err != nil {
		reqBodyStr = "failed to parse body or body was empty"
	} else {
		reqBodyStr = fmt.Sprintf("msg: %s | ts: %d", reqBody.Message, reqBody.Timestamp)
	}

	params := ""
	for k, v := range req.Params {
		params += fmt.Sprintf("%s=%s | ", k, v)
	}

	fmt.Printf("Method:  %s\n", method)
	fmt.Printf("Path:    %s\n", path)
	fmt.Printf("Params:  %s\n", params)
	fmt.Printf("Body:    %s\n", reqBodyStr)
	fmt.Printf("RawBody: %s\n\n", req.Body)

	body := map[string]string{
		"Method": method,
		"Path":   path,
		"Params": params,
		"Body":   reqBodyStr,
	}

	resBody, err := json.Marshal(body)
	if err != nil {
		status = response.StatusInternalServerError
		resBody = []byte("failed to jsonify output")
		h.Replace("Content-Type", "text/plain")
	}

	h.Replace("Content-Length", strconv.Itoa(len(resBody)))
	err = w.WriteResponse(status, h, resBody)
	return err
}

func main() {
	// Set route handlers
	r := router.NewRouter()
	r.GET("/", serveIndex)
	r.GET("/index.html", serveIndex)
	r.GET("/favicon.ico", serveFavicon)
	r.GET("/styles.css", serveStyles)
	r.GET("/app.js", serveApp)

	r.GET("/api/echo", echo)
	r.POST("/api/echo", echo)
	r.PUT("/api/echo", echo)
	r.DELETE("/api/echo", echo)
	r.PATCH("/api/echo", echo)

	r.GET("/api/echo?&id", echoParams)
	r.POST("/api/echo?&id", echoParams)
	r.PUT("/api/echo?&id", echoParams)
	r.DELETE("/api/echo?&id", echoParams)

	r.GET("/api/echo?&id&flag", echoParams)
	r.POST("/api/echo?&id&flag", echoParams)
	r.DELETE("/api/echo?&id&flag", echoParams)

	r.GET("/api/echo?&id&flag&test", echoParams)
	r.POST("/api/echo?&id&flag&test", echoParams)

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
