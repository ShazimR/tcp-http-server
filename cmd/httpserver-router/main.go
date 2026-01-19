package httpserverrouter

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ShazimR/tcp-http-server/internal/request"
	"github.com/ShazimR/tcp-http-server/internal/response"
	"github.com/ShazimR/tcp-http-server/internal/router"
	"github.com/ShazimR/tcp-http-server/internal/server"
)

const port = 8080

// Example route handlers
func rootGetHandler(w *response.Writer, req *request.Request) error  { return nil }
func rootPostHandler(w *response.Writer, req *request.Request) error { return nil }
func chunkGetHandler(w *response.Writer, req *request.Request) error { return nil }
func videoGetHandler(w *response.Writer, req *request.Request) error { return nil }

func main() {
	// Set route handlers
	r := router.NewRouter()
	r.GET("/", rootGetHandler)
	r.POST("/", rootPostHandler)
	r.GET("/chunk", chunkGetHandler)
	r.GET("/video", videoGetHandler)

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
