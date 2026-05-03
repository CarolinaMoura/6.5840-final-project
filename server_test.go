// Manual testing:
//
// [Should create a room]
//     - in one terminal:
//         > go run .
//         > curl -X POST http://localhost:8080/api/rooms
//  Expected: {"room":"<room>"}
//
//
//  [Should upgrade connection to a websocket + should discover peers correctly]
//      - in one terminal:
//        > go run .
//        > curl -X POST http://localhost:8080/api/rooms
//      - (make sure you have websocat installed)
//        > websocat ws://localhost:8080/api/ws/rooms/<room>
//   Expected: `{"id":"<uuid1>","peers":<[]>,"type":"welcome"}`
//      - in another terminal:
//        > websocat ws://localhost:8080/api/ws/rooms/<room>
//   Expected: `{"id":"<uuid2>","peers":<[uuid1]>,"type":"welcome"}`
//      - in the first terminal:
//   Expected: `{"id":"<uuid2>","type":"peer-joined"}`

package main

import (
	"net/http/httptest"
	"testing"

	"6.5840-final-project/clerk"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func makeTestServer(t *testing.T, rsmClusterSize int) (*Server, func()) {
	t.Helper()
	ck, cleanup := clerk.MakeTestClerk(t, rsmClusterSize, integration.WithoutGoLeakDetection())
	s := &Server{
		rooms: make(map[string]map[string]*peer),
		app:   fiber.New(),
		ck:    ck,
	}
	s.registerRoutes()
	return s, cleanup
}

func TestCreateRoom(t *testing.T) {
	s, cleanup := makeTestServer(t, 1)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/rooms", nil)
	resp, _ := s.app.Test(req)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
}
