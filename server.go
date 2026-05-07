package main

// [TODO] maybe change Server.rooms to be a package RoomManager?
// Disgusting code; can we ever change to TypeScript please 🥹

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"slices"
	"strconv"
	"sync"

	"6.5840-final-project/clerk"
	"6.5840-final-project/kv"
	"6.5840-final-project/kv/kvpb"
	"6.5840-final-project/raft"
	"6.5840-final-project/raft/persister"
	pb "6.5840-final-project/raft/raftpb"
	"6.5840-final-project/rsm"
	"6.5840-final-project/rsm/rpc"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	mu            sync.RWMutex
	rooms         map[string]map[string]*peer // room -> peerID -> peer
	app           *fiber.App
	listenAddr    string // where to bind (e.g. "0.0.0.0:8080")
	advertiseAddr string // how peers/clients reach us (e.g. "localhost:8082")
	ck            *clerk.Clerk
	kv            *kv.KVServer       // local replicated kv (will replace etcd)
	raftShutdown  func()             // tears down raft gRPC + persister
	peerConns     []*grpc.ClientConn // shared by raft + clerk; closed in Shutdown
}

// Checks if a room is valid (6 alphanumeric chars).
func isRoomValid(room string) bool {
	return len(room) == 6 && isAlphaNumericString(room)
}

// POST /rooms — creates a new room with a fresh 6-char alphanumeric code.
func (s *Server) handleCreateRoom(c fiber.Ctx) error {
	allServers, err := s.ck.Signalings()

	if err != rpc.OK {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get other servers"})
	}
	if len(allServers) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "No signaling servers available"})
	}

	assignedServer := allServers[rand.Intn(len(allServers))]

	var room string

	for {
		room = generateAlphanumericString(6)
		err := s.ck.Put("room/"+room, assignedServer, 0)

		if err == rpc.OK {
			break
		}
	}

	fmt.Printf("Room %q created\n", room)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"room": room, "server": assignedServer})
}

// GET /rooms/:room — returns the signaling server assigned to this room.
// If the assigned server died (the lease expired), reassigns the room to a
// live signaling and returns that.
// @throws 404 if room doesn't exist
// @throws 503 if no signalings are alive to reassign to
func (s *Server) handleLookupRoom(c fiber.Ctx) error {
	room := c.Params("room")

	server, err := s.assignedSignaling(room)
	switch err {
	case rpc.OK:
		return c.JSON(fiber.Map{"server": server})
	case rpc.ErrNoKey:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "room not found"})
	default:
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "no signaling available"})
	}
}

// Returns the signaling currently responsible for `room`. If the recorded
// signaling is no longer in the live set (its lease has expired), picks a
// live signaling at random and room/<room> to point at it.
// Loops on ErrVersion in case another lookup raced us to reassign.
func (s *Server) assignedSignaling(room string) (string, rpc.Err) {
	for {
		// Get current assigned server
		server, version, getErr := s.ck.Get("room/" + room)
		if getErr != rpc.OK {
			return "", getErr
		}

		// All current alive
		live, listErr := s.ck.Signalings()
		if listErr != rpc.OK {
			return "", listErr
		}
		if slices.Contains(live, server) { // if the assigned server is in the live list, return it
			return server, rpc.OK
		}
		if len(live) == 0 { // no live servers to reassign to
			return "", rpc.ErrEtcd
		}

		// The server is dead, pick a new random server
		newServer := live[rand.Intn(len(live))]
		putErr := s.ck.Put("room/"+room, newServer, version)

		if putErr == rpc.OK {
			fmt.Printf("Room %q reassigned: %q (dead) -> %q\n", room, server, newServer)
			return newServer, rpc.OK
		}
		if putErr == rpc.ErrVersion { // someone raced us; prob will get out in the next iteration
			continue
		}
		return "", putErr
	}
}

// GET /ws/rooms/:room — joins the mesh signaling channel for the room.
func (s *Server) handleJoinRoom(ws *websocket.Conn) {
	room := ws.Params("room")
	p := &peer{id: uuid.NewString(), conn: ws}

	s.mu.Lock()
	// Lazy-init: rooms are durably tracked in etcd, but the in-memory
	// peer set for a room only exists once someone joins this signaling.
	if s.rooms[room] == nil {
		s.rooms[room] = make(map[string]*peer)
	}
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
	if len(s.rooms[room]) == 0 {
		delete(s.rooms, room)
	}
	s.mu.Unlock()
}

// Pre-upgrade gate for /ws/rooms/:room — validates room exists before upgrading.
// @throws 400 if room is malformed
// @throws 404 if room doesn't exist
func (s *Server) wsGate(c fiber.Ctx) error {
	room := c.Params("room")

	// Is room malformed?
	if !isRoomValid(room) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "room is not a valid format"})
	}

	// Does room exist?
	server, _, err := s.ck.Get("room/" + room)
	if err != rpc.OK {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "room not found"})
	}

	// Am I the assigned server?
	if server != s.advertiseAddr {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "wrong server for this room"})
	}

	if !websocket.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}
	return c.Next()
}

func (s *Server) registerRoutes() {
	s.app.Post("/api/rooms", s.handleCreateRoom)
	s.app.Get("/api/rooms/:room", s.handleLookupRoom)

	s.app.Use("/api/ws/rooms/:room", s.wsGate)
	s.app.Get("/api/ws/rooms/:room", websocket.New(s.handleJoinRoom))

	s.app.Use("/", static.New("./web/dist"))
	s.app.Get("/*", func(c fiber.Ctx) error {
		return c.SendFile("./web/dist/index.html")
	})
}

// Blocks until the server stops
func (s *Server) Start() error {
	return s.app.Listen(s.listenAddr)
}

func (s *Server) Shutdown() error {
	if s.raftShutdown != nil {
		s.raftShutdown()
	}
	for _, c := range s.peerConns {
		_ = c.Close()
	}
	return s.app.Shutdown()
}

// dialPeers opens one gRPC client connection per address. The returned
// conns can host multiple typed stubs (raft + kv), so the same TCP/H2
// connection serves both services rather than each opening its own.
func dialPeers(addrs []string) ([]*grpc.ClientConn, error) {
	conns := make([]*grpc.ClientConn, len(addrs))
	for i, addr := range addrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			for _, c := range conns {
				if c != nil {
					_ = c.Close()
				}
			}
			return nil, fmt.Errorf("dial peer %d (%s): %w", i, addr, err)
		}
		conns[i] = conn
	}
	return conns, nil
}

// conns is one gRPC client connection per peer (in cluster index order);
// listenAddr is where this node listens for incoming raft+kv traffic.
//
// extraRegister is called after the raft service is registered but before
// Serve starts, so callers can multiplex other gRPC services (e.g. kv) on
// the same listener. May be nil.
//
// Returns the RSM and a shutdown func that stops gRPC and closes the
// persister. Connection cleanup is the caller's responsibility.
func bootRaft(me int, conns []*grpc.ClientConn, listenAddr, dataDir string, sm rsm.StateMachine, extraRegister func(*grpc.Server)) (*rsm.RSM, func(), error) {
	if me < 0 || me >= len(conns) {
		return nil, nil, fmt.Errorf("me=%d out of range for %d peers", me, len(conns))
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen %s: %w", listenAddr, err)
	}

	clients := make([]pb.RaftClient, len(conns))
	for i, c := range conns {
		clients[i] = pb.NewRaftClient(c)
	}

	// On-disk persister for raft log + snapshot.
	pst, err := persister.New(fmt.Sprintf("%s/raft-%d.db", dataDir, me))
	if err != nil {
		_ = lis.Close()
		return nil, nil, fmt.Errorf("persister: %w", err)
	}

	rsmInst := rsm.MakeRSM(clients, me, pst, -1, sm)
	rfServer, ok := rsmInst.Raft().(*raft.Raft)
	if !ok {
		_ = lis.Close()
		_ = pst.Close()
		return nil, nil, fmt.Errorf("rsm.Raft() did not return *raft.Raft")
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterRaftServer(grpcSrv, rfServer)
	if extraRegister != nil {
		extraRegister(grpcSrv)
	}

	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("raft grpc server stopped: %v", err)
		}
	}()

	shutdown := func() {
		grpcSrv.GracefulStop()
		_ = pst.Close()
	}
	return rsmInst, shutdown, nil
}

func makeServer(port int, raftPeers []string) *Server {
	advertise := os.Getenv("ADVERTISE_ADDR")
	if advertise == "" {
		log.Fatal("ADVERTISE_ADDR is required")
	}

	meStr := os.Getenv("ME")
	if meStr == "" {
		log.Fatal("ME is required (this node's index into the raft peer list)")
	}
	me, err := strconv.Atoi(meStr)
	if err != nil {
		log.Fatalf("invalid ME=%q: %v", meStr, err)
	}
	dataDir := os.Getenv("RAFT_DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}

	// Dial every peer once; raft and the clerk share the connections so
	// each peer relationship uses one TCP/H2 link, not two.
	peerConns, err := dialPeers(raftPeers)
	if err != nil {
		log.Fatal("dialPeers:", err)
	}

	// Start KV server. Registered as a gRPC service on the same listener
	// as raft (multiplexed via the extraRegister hook), so the same
	// connections in peerConns serve both raft and kv traffic.
	kvSM := kv.NewKVServer(me)
	rsmInst, raftShutdown, err := bootRaft(me, peerConns, raftPeers[me], dataDir, kvSM, func(s *grpc.Server) {
		kvpb.RegisterKVServer(s, kvSM)
	})
	if err != nil {
		log.Fatal("bootRaft:", err)
	}
	kvSM.Bind(rsmInst)

	// Create clerk. The list of live signalings is now discovered
	// dynamically via heartbeats in the kv (see ck.RegisterWithLease /
	// ck.Signalings) — no static SIGNALING_ADDRS env var needed.
	ck, err := clerk.MakeClerk(peerConns)
	if err != nil {
		log.Fatal("MakeClerk:", err)
	}

	s := &Server{
		rooms:         make(map[string]map[string]*peer),
		app:           fiber.New(),
		listenAddr:    fmt.Sprintf("0.0.0.0:%d", port),
		advertiseAddr: advertise,
		kv:            kvSM,
		raftShutdown:  raftShutdown,
		ck:            ck,
		peerConns:     peerConns,
	}

	s.registerRoutes()

	// Start heartbeating advertise addr
	stopLease := ck.RegisterWithLease(advertise, 30)
	prev := s.raftShutdown
	s.raftShutdown = func() {
		stopLease()
		if prev != nil {
			prev()
		}
	}
	return s
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: go run server.go <port> <raft-server1> <raft-server2> ...")
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal("Invalid port:", err)
	}
	server := makeServer(port, os.Args[2:])
	if err := server.Start(); err != nil {
		log.Fatal("Server crashed:", err)
	}
}
