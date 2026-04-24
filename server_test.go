package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestHandleConnections_Echo(t *testing.T) {
	server := makeServer()
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"

	fmt.Println(wsURL)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("could not open a ws connection on %s %v", wsURL, err)
	}
	defer ws.Close()

	messageToSend := []byte("hello server")
	if err := ws.WriteMessage(websocket.TextMessage, messageToSend); err != nil {
		t.Fatalf("could not send message: %v", err)
	}

	_, messageReceived, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("could not read message: %v", err)
	}

	if string(messageReceived) != string(messageToSend) {
		t.Errorf("expected %s, got %s", messageToSend, messageReceived)
	}
}
