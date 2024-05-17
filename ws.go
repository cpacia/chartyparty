package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
)

type JoinMsg struct {
	Join struct {
		UserID string `json:"userID"`
		GameID string `json:"gameID"`
	} `json:"join"`
}

type connection struct {
	// The websocket connection
	ws *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// The hub
	h *hub
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err) {
			}
			break
		}
		joinMsg := &JoinMsg{}
		if err := json.Unmarshal(message, joinMsg); err != nil {
			continue
		}
		game, ok := c.h.server.games[joinMsg.Join.GameID]
		if !ok {
			continue
		}

		if joinMsg.Join.UserID == game.player1ID {
			game.conn1 = c
			if game.conn2 != nil {
				game.conn2.send <- []byte(fmt.Sprintf(`{"connect": "%s"}`, game.player1Name))
			}
		}
		if joinMsg.Join.UserID == game.player2ID {
			game.conn2 = c
			if game.conn1 != nil {
				game.conn1.send <- []byte(fmt.Sprintf(`{"connect": "%s"}`, game.player2Name))
			}
		}
		fmt.Printf("Websocket. User: %s joined game: %s\n", joinMsg.Join.UserID, joinMsg.Join.GameID)
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type hub struct {
	// Registered connections
	connections map[*connection]bool

	// Outbound messages to the connections
	Broadcast chan []byte

	// Register requests from the connections
	register chan *connection

	// Unregister requests from connections
	unregister chan *connection

	server *server
}

func newHub() *hub {
	return &hub{
		Broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		case m := <-h.Broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					close(c.send)
				}
			}
		}
	}
}

type websocketHandler struct {
	hub *hub
}

func newWebsocketHandler(hub *hub) *websocketHandler {
	handler := websocketHandler{
		hub: hub,
	}
	return &handler
}

func (wsh websocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &connection{send: make(chan []byte, 256), ws: ws, h: wsh.hub}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
}
