package main

// [TODO] maybe change Server.rooms to be a package RoomManager?
// Disgusting code; can we ever change to TypeScript please 🥹

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Server struct {
	mu    sync.RWMutex
	rooms map[string][]*websocket.Conn
}

// Checks if a room is valid (6 alphanumeric chars).
func isRoomValid(room string) bool {
	return len(room) == 6 && isAlphaNumericString(room)
}

// POST /rooms — creates a new room and returns its name.
// @throws 400 if request body is malformed
// @throws 405 if method is not allowed (not POST)
// @throws 409 if room already exists
func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	// Throws 405 if method is not POST
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Room string `json:"room"`
	}

	// Throws 400 if request body is malformed
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !isRoomValid(body.Room) {
		http.Error(w, `{"error":"room name required and must be 6 alphanumeric"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	// Throws 409 if room already exists
	if _, exists := s.rooms[body.Room]; exists {
		s.mu.Unlock()
		http.Error(w, `{"error":"room already exists"}`, http.StatusConflict)
		return
	}
	s.rooms[body.Room] = []*websocket.Conn{}
	s.mu.Unlock()

	fmt.Printf("Room %q created\n", body.Room)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"room": body.Room})
}

// GET /ws/rooms/:room — upgrades to WebSocket and joins the room.
// @throws 400 if room is malformed
// @throws 404 if room doesn't exist
func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request, room string) {
	s.mu.RLock()
	_, exists := s.rooms[room]
	s.mu.RUnlock()

	// Throws 400 if room is malformed
	if !isRoomValid(room) {
		http.Error(w, `{"error":"room is not valid"}`, http.StatusBadRequest)
		return
	}

	// Throws 404 if room doesn't exist
	if !exists {
		http.Error(w, `{"error":"room not found"}`, http.StatusNotFound)
		return
	}

	// Takes a normal HTTP connection and upgrades it to a WebSocket
	// [TODO] The buffer sizes are default (i.e., set to 0);
	// we should play with them if performance demands it
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  0,
		WriteBufferSize: 0,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, `{"error":"failed to upgrade connection"}`, http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.rooms[room] = append(s.rooms[room], ws)
	s.mu.Unlock()

	defer s.removeFromRoom(room, ws)
	defer ws.Close()

	for {
		messageType, payload, err := ws.ReadMessage()

		if err != nil { // Client disconnected?
			s.removeFromRoom(room, ws)
			break
		}

		s.broadcast(room, ws, messageType, payload)
	}
}

func (s *Server) removeFromRoom(room string, ws *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conns := s.rooms[room]
	for i, c := range conns {
		if c == ws {
			s.rooms[room] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
}

func (s *Server) broadcast(room string, sender *websocket.Conn, messageType int, payload []byte) {
	// TODO
}

func makeServer() *Server {
	return &Server{
		rooms: make(map[string][]*websocket.Conn),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/rooms":
		s.handleCreateRoom(w, r)

	case strings.HasPrefix(r.URL.Path, "/ws/rooms/"):
		room := strings.TrimPrefix(r.URL.Path, "/ws/rooms/")
		s.handleJoinRoom(w, r, room)

	default:
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
