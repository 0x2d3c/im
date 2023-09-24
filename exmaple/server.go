package main

import (
	"log"
	"net/http"

	v0 "im/v0"
	v1 "im/v1"
)

func main() {
	// HTTP handler for WebSocket connections.
	http.HandleFunc("/ws/v0", v0.HandleWebSocket)
	http.HandleFunc("/ws/v1", v1.HandleWebSocket)

	// Start the HTTP server on port 8443.
	log.Fatal(http.ListenAndServe(":8443", nil))
}
