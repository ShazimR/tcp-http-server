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
	"strings"
	"syscall"
	"time"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/ShazimR/tcp-http-server/internal/router"
	"github.com/ShazimR/tcp-http-server/internal/server"
)

const port = 8080
const KiB = 1024 // bytes
const maxChunkSize = 32 * KiB
const testAuthKey = "some-key-for-now"

type TestResponse struct {
	Message   string `json:"msg"`
	Timestamp uint64 `json:"ts"`
}

type LoginResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func parseDemoBody(req *request.Request) string {
	var reqBody TestResponse
	if err := json.Unmarshal(req.Body, &reqBody); err != nil {
		return "failed to parse body or body was empty"
	}
	return fmt.Sprintf("msg: %s | ts: %d", reqBody.Message, reqBody.Timestamp)
}

func serveStatic(filename string, contentType string, req *request.Request, w *response.Writer) error {
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", contentType)

	f, err := os.Open(filename)
	if err != nil {
		body := []byte("error loading content")
		h.Replace("Content-Type", "text/plain")
		h.Replace("Content-Length", strconv.Itoa(len(body)))
		return w.WriteResponse(response.StatusInternalServerError, h, body)
	}
	defer f.Close()

	if _, ok := req.Headers.Get("Range"); ok {
		info, err := f.Stat()
		if err != nil {
			body := []byte("error loading content")
			h.Replace("Content-Type", "text/plain")
			h.Replace("Content-Length", strconv.Itoa(len(body)))
			return w.WriteResponse(response.StatusInternalServerError, h, body)
		}

		filesize := int(info.Size())
		return w.WritePartialContentResponse(f, filesize, contentType, req)
	}

	body, err := io.ReadAll(f)
	if err != nil {
		body = []byte("error loading file")
		h.Replace("Content-Type", "text/plain")
	}

	h.Replace("Content-Length", strconv.Itoa(len(body)))
	return w.WriteResponse(response.StatusOK, h, body)
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
	return serveStatic("./static/index.html", "text/html", req, w)
}

func serveFavicon(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/favicon.ico", "image/x-icon", req, w)
}

func serveStyles(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/styles.css", "text/css", req, w)
}

func serveApp(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/app.js", "text/javascript", req, w)
}

func serveVideo(w *response.Writer, req *request.Request) error {
	return serveStatic("./static/one-last-breath.mp4", "video/mp4", req, w)
}

func serveVideoChunked(w *response.Writer, req *request.Request) error {
	return serveChunked("./static/one-last-breath.mp4", "video/mp4", w)
}

func echo(w *response.Writer, req *request.Request) error {
	status := response.StatusOK
	h := response.GetDefaultHeaders(0)
	h.Replace("Content-Type", "application/json")

	method := req.RequestLine.Method
	path := req.RequestLine.RequestTarget
	bodyStr := parseDemoBody(req)

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

func login(w *response.Writer, req *request.Request) error {
	var reqBody LoginResponse
	if err := json.Unmarshal(req.Body, &reqBody); err != nil {
		body := []byte("body must include 'username' and 'password' keys")
		h := response.GetDefaultHeaders(len(body))
		h.Replace("Content-Type", "text/plain")
		return w.WriteResponse(response.StatusBadRequest, h, body)
	}

	const testUsername = "shazimr"
	const testPassword = "password123"

	if reqBody.Username != testUsername || reqBody.Password != testPassword {
		body := []byte("username or password is incorrect (username is 'shazimr' and password is 'password123')")
		h := response.GetDefaultHeaders(len(body))
		h.Replace("Content-Type", "text/plain")
		return w.WriteResponse(response.StatusUnauthorized, h, body)
	}

	h := response.GetDefaultHeaders(0)
	maxAge := 120 // seconds
	response.SetCookie(h, "Authentication", testAuthKey, fmt.Sprintf("Max-Age=%d", maxAge), "Samesite=Lax")
	return w.WriteResponse(response.StatusOK, h, []byte{})
}

func logger(next response.Handler) response.Handler {
	return func(w *response.Writer, req *request.Request) error {
		reqTime := time.Now()
		err := next(w, req)
		errStr := ""
		if err != nil {
			errStr = fmt.Sprintf("Error from handler: %v\n\n", err)
		}
		resTime := time.Now()

		method := req.RequestLine.Method
		path := req.RequestLine.RequestTarget
		headerStr := ""
		req.Headers.ForEach(func(name, value string) {
			headerStr += fmt.Sprintf("  - %s: %s\n", name, value)
		})
		contentType, _ := req.Headers.Get("Content-Type")

		// Prevent printing long bodies
		const maxLogBody = 4096
		body := req.Body
		if len(body) > maxLogBody {
			body = body[:maxLogBody]
		}

		resStat, resH, resB := w.GetLogs()
		resH, _ = strings.CutSuffix(resH, "\r\n")
		resS := w.GetStatus()
		elapsedTime := resTime.Sub(reqTime)

		var elapsedTimeStr string
		if elapsedTime.Milliseconds() > 0 {
			elapsedTimeStr = fmt.Sprintf("%d ms", elapsedTime.Milliseconds())
		} else {
			elapsedTimeStr = fmt.Sprintf("%d us", elapsedTime.Microseconds())
		}

		// ANSI colour codes
		const (
			blue  = "\033[34m"
			green = "\033[32m"
			red   = "\033[31m"
			reset = "\033[0m"
		)

		var logs strings.Builder
		fmt.Fprint(&logs, blue)
		fmt.Fprintf(&logs, "%s  Request:\n", reqTime.Format(time.RFC1123))
		fmt.Fprint(&logs, reset)
		fmt.Fprintf(&logs, "Method:      %s\n", method)
		fmt.Fprintf(&logs, "Path:        %s\n", path)
		fmt.Fprintf(&logs, "PathParams:  %s\n", req.PathParams)
		fmt.Fprintf(&logs, "QueryParams: %s\n", req.RequestParams)
		fmt.Fprintf(&logs, "Headers:\n%s", headerStr)
		if contentType != "application/json" && !strings.HasPrefix(contentType, "text/") {
			// Prevent printing binary directly as a string
			fmt.Fprintf(&logs, "Body (%d bytes, showing %d):\n% x\n\n", len(req.Body), len(body), body)
		} else {
			fmt.Fprintf(&logs, "Body (%d bytes, showing %d):\n%s\n\n", len(req.Body), len(body), body)
		}

		if errStr != "" {
			fmt.Fprint(&logs, red)
			fmt.Fprintf(&logs, "%s Request Handling Error: %s\n\n", resTime.Format(time.RFC1123), errStr)
			fmt.Fprint(&logs, reset)
		}

		fmt.Fprint(&logs, green)
		fmt.Fprintf(&logs, "%s  Response (%d):\n", resTime.Format(time.RFC1123), resS)
		fmt.Fprint(&logs, reset)
		fmt.Fprintf(&logs, "%s%s", resStat, resH)
		fmt.Fprintf(&logs, "Length of Body: %d\n", len(resB))
		fmt.Fprintf(&logs, "Total Elapsed Time: %s\n\n", elapsedTimeStr)

		printLogs(logs.String())
		return err
	}
}

func printLogs(logs string) {
	log.Print(logs + "----------------------------------------------------------------" +
		"--------------------------------\n\n")
}

func auth(next response.Handler) response.Handler {
	return func(w *response.Writer, req *request.Request) error {
		if authKey, ok := req.GetCookie("Authentication"); ok && authKey == testAuthKey {
			return next(w, req)

		} else {
			body := []byte("please login to access")
			h := response.GetDefaultHeaders(len(body))
			h.Replace("Content-Type", "text/plain")
			return w.WriteResponse(response.StatusUnauthorized, h, body)
		}
	}
}

func force400(_ response.Handler) response.Handler {
	return func(w *response.Writer, req *request.Request) error {
		return w.WriteResponse(response.StatusBadRequest, response.GetDefaultHeaders(0), []byte{})
	}
}

func main() {
	// Routers
	r := router.NewRouter()
	r.Use(logger, force400)
	api := r.Group("/api")
	api.Use(auth)
	echoRouter := api.Group("/echo")
	userPosts := api.Group("/users/:userid/posts/:postid")

	// Static assets
	r.GET("/", serveIndex)
	r.GET("/index.html", serveIndex)
	r.GET("/favicon.ico", serveFavicon)
	r.GET("/styles.css", serveStyles)
	r.GET("/app.js", serveApp)
	r.GET("/video", serveVideo)
	r.GET("/video-chunked", serveVideoChunked)

	// Auth routes
	r.POST("/login", login)

	// Base echo (no path params needed)
	// Example target /api/echo?flag=true&test=a=b
	echoRouter.GET("/", echo)
	echoRouter.POST("/", echo)
	echoRouter.PUT("/", echo)
	echoRouter.DELETE("/", echo)
	echoRouter.PATCH("/", echo)

	// Path param-aware echo
	// Example target /api/echo/123?flag=true&test=a=b
	echoRouter.GET("/:id", echoParams)
	echoRouter.POST("/:id", echoParams)
	echoRouter.PUT("/:id", echoParams)
	echoRouter.DELETE("/:id", echoParams)
	echoRouter.PATCH("/:id", echoParams)

	// Multi path param example
	// Example target /api/users/7/posts/99?view=full
	userPosts.GET("/", echoParams)
	userPosts.POST("/", echoParams)

	// Setup log
	log.SetOutput(os.Stdout) // use stdout instead of stderr
	log.SetFlags(0)          // don't add date/time as the logger handles this

	// Setup and run server
	s, err := server.Serve(port, nil, r)
	if err != nil {
		log.Fatalf("error starting server: %v", err)
	}
	defer s.Close()
	log.Printf("Server started on port: %d\n\n", port)

	// Stop server gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}
