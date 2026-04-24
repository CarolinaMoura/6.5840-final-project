package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Server struct {
	mu    sync.RWMutex
	rooms map[int][]*websocket.Conn
}

// Takes a normal HTTP connection and upgrades it to a WebSocket
// [TODO] The buffer sizes are default; we should play with them
// if performance demands it
var upgrader = websocket.Upgrader{
	ReadBufferSize:  0,
	WriteBufferSize: 0,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}

	defer ws.Close()

	fmt.Println("New peer connected successfully!")

	for {
		messageType, payload, err := ws.ReadMessage()
		if err != nil {
			log.Println("Client disconnected or error:", err)
			break
		}

		fmt.Printf("Received message: %s\n", payload)

		err = ws.WriteMessage(messageType, payload)
		if err != nil {
			log.Println("Failed to write message:", err)
			break
		}
	}
}

func makeServer() *Server {
	return &Server{
		rooms: make(map[int][]*websocket.Conn),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/ws" {
		handleConnections(w, r)
	} else {
		http.ServeFile(w, r, "index.html")
	}
}

func main() {
	server := makeServer()

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: server,
	}

	fmt.Println("Server listening on http://localhost:8080")
	err := httpSrv.ListenAndServe()
	if err != nil {
		log.Fatal("Server crashed:", err)
	}
}
