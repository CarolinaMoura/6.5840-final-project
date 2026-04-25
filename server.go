package main

// [TODO] maybe change Server.rooms to be a package RoomManager?
// Disgusting code; can we ever change to TypeScript please 🥹

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type peer struct {
	mu   sync.Mutex
	id   string
	conn *websocket.Conn
}

func (p *peer) send(payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conn.WriteMessage(websocket.TextMessage, payload)
}

func (p *peer) sendJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.send(payload)
}

type Server struct {
	mu    sync.RWMutex
	rooms map[string]map[string]*peer // room -> peerID -> peer
	app   *fiber.App
	addr  string
}

// Checks if a room is valid (6 alphanumeric chars).
func isRoomValid(room string) bool {
	return len(room) == 6 && isAlphaNumericString(room)
}

// POST /rooms — creates a new room with a fresh 6-char alphanumeric code.
func (s *Server) handleCreateRoom(c fiber.Ctx) error {
	var room string

	s.mu.Lock()
	for {
		room = generateAlphanumericString(6)
		if _, exists := s.rooms[room]; !exists {
			s.rooms[room] = make(map[string]*peer)
			break
		}
	}
	s.mu.Unlock()

	fmt.Printf("Room %q created\n", room)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"room": room})
}

// GET /ws/rooms/:room — joins the mesh signaling channel for the room.
func (s *Server) handleJoinRoom(ws *websocket.Conn) {
	room := ws.Params("room")
	p := &peer{id: uuid.NewString(), conn: ws}

	s.mu.Lock()
	existingPeers := make([]string, 0, len(s.rooms[room]))
	for id := range s.rooms[room] {
		existingPeers = append(existingPeers, id)
	}
	s.rooms[room][p.id] = p
	s.mu.Unlock()

	welcome := fiber.Map{"type": "welcome", "id": p.id, "peers": existingPeers}
	if err := p.sendJSON(welcome); err != nil {
		s.removePeer(room, p.id)
		ws.Close()
		return
	}

	s.broadcastControl(room, p.id, fiber.Map{"type": "peer-joined", "id": p.id})

	defer s.removePeer(room, p.id)
	defer s.broadcastControl(room, p.id, fiber.Map{"type": "peer-left", "id": p.id})
	defer ws.Close()

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			break
		}
		s.routeMessage(room, p.id, payload)
	}
}

// Forwards a peer-originated message. Stamps "from"; if "to" is
// set and matches a peer in the room, unicasts; otherwise broadcasts to all
// other peers. Drops silently on malformed JSON or unknown target.
func (s *Server) routeMessage(room, fromID string, payload []byte) {
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	msg["from"] = fromID
	out, err := json.Marshal(msg)
	if err != nil {
		return
	}

	to, _ := msg["to"].(string)
	s.deliver(room, to, fromID, out)
}

// Sends a server-generated control message to all peers in
// the room except `excludeID`.
func (s *Server) broadcastControl(room, excludeID string, msg fiber.Map) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.deliver(room, "", excludeID, payload)
}

// Sends payload to peers in `room`. If targetID is non-empty and that
// peer exists, unicasts; otherwise fans out to everyone except `excludeID`.
func (s *Server) deliver(room, targetID, excludeID string, payload []byte) {
	if targetID != "" {
		s.mu.RLock()
		target := s.rooms[room][targetID]
		s.mu.RUnlock()
		if target != nil {
			target.send(payload)
		}
		return
	}
	for _, pr := range s.snapshotPeers(room, excludeID) {
		pr.send(payload)
	}
}

// Returns the peers in `room` excluding `excludeID`, copied out
// from under the lock so callers can send without blocking room mutations.
func (s *Server) snapshotPeers(room, excludeID string) []*peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peers := s.rooms[room]
	out := make([]*peer, 0, len(peers))
	for id, pr := range peers {
		if id != excludeID {
			out = append(out, pr)
		}
	}
	return out
}

func (s *Server) removePeer(room, id string) {
	s.mu.Lock()
	delete(s.rooms[room], id)
	s.mu.Unlock()
}

// Pre-upgrade gate for /ws/rooms/:room — validates room exists before upgrading.
// @throws 400 if room is malformed
// @throws 404 if room doesn't exist
func (s *Server) wsGate(c fiber.Ctx) error {
	room := c.Params("room")

	if !isRoomValid(room) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "room is not a valid format"})
	}

	s.mu.RLock()
	_, exists := s.rooms[room]
	s.mu.RUnlock()

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "room not found"})
	}

	if !websocket.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}
	return c.Next()
}

func (s *Server) registerRoutes() {
	s.app.Post("/rooms", s.handleCreateRoom)

	s.app.Use("/ws/rooms/:room", s.wsGate)
	s.app.Get("/ws/rooms/:room", websocket.New(s.handleJoinRoom))

	s.app.Get("/*", func(c fiber.Ctx) error {
		return c.SendFile("index.html")
	})
}

// Blocks until the server stops
func (s *Server) Start() error {
	return s.app.Listen(s.addr)
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

func makeServer(port int) *Server {
	s := &Server{
		rooms: make(map[string]map[string]*peer),
		app:   fiber.New(),
		addr:  fmt.Sprintf(":%d", port),
	}
	s.registerRoutes()
	return s
}

func main() {
	server := makeServer(8080)
	if err := server.Start(); err != nil {
		log.Fatal("Server crashed:", err)
	}
}
