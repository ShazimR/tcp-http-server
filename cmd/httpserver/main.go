package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ShazimR/tcp-http-server/internal/server"
)

const port = 8080

func main() {
	s, err := server.Serve(port)
	if err != nil {
		log.Fatalf("error starting server: %v", err)
	}
	defer s.Close()
	log.Println("Server started on port: ", port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}
