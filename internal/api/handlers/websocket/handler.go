package websocket

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"print-pro-backend/internal/infrastructure"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins - adjust in production
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub         *Hub
	redisClient *infrastructure.RedisClient
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *Hub, redisClient *infrastructure.RedisClient) *WebSocketHandler {
	return &WebSocketHandler{
		hub:         hub,
		redisClient: redisClient,
	}
}

// HandleWebSocket handles WebSocket connections at /ws/{printer_id}
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract printer_id from URL path
	path := r.URL.Path
	if len(path) <= 4 || path[:4] != "/ws/" {
		http.Error(w, "Invalid WebSocket path", http.StatusBadRequest)
		return
	}
	
	printerID := path[4:] // Skip "/ws/"
	if printerID == "" {
		http.Error(w, "printer_id is required", http.StatusBadRequest)
		return
	}

	log.Printf("INFO: WebSocket connection attempt from printer_id: '%s'", printerID)

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERROR: Failed to upgrade connection for printer_id %s: %v", printerID, err)
		return
	}

	log.Printf("SUCCESS: WebSocket connection established for printer_id: '%s'", printerID)

	// Register connection in hub
	connection := h.hub.Register(printerID, conn)
	defer func() {
		h.hub.Unregister(printerID)
		conn.Close()
		log.Printf("INFO: WebSocket connection closed for printer_id: '%s'", printerID)
	}()

	ctx := r.Context()

	// Set printer status to "online" in Redis with 60-second TTL
	redisKey := "printer:" + printerID + ":status"
	if err := h.redisClient.Set(ctx, redisKey, "online", 60*time.Second); err != nil {
		log.Printf("ERROR: Failed to set Redis key for printer_id %s: %v", printerID, err)
	}

	// Set Pong handler to refresh Redis TTL when Pong is received
	conn.SetPongHandler(func(string) error {
		log.Printf("DEBUG: Pong received from printer_id: '%s', refreshing Redis TTL", printerID)
		ctx := context.Background()
		if err := h.redisClient.Set(ctx, redisKey, "online", 60*time.Second); err != nil {
			log.Printf("ERROR: Failed to refresh Redis TTL for printer_id %s: %v", printerID, err)
		}
		conn.SetReadDeadline(time.Now().Add(25 * time.Second))
		return nil
	})

	// Set initial read deadline (expect Pong within 25 seconds)
	conn.SetReadDeadline(time.Now().Add(25 * time.Second))

	// Start goroutine to send Ping messages every 20 seconds
	go h.pingLoop(connection, redisKey)

	// Start goroutine to write messages
	go h.writePump(connection)

	// Read messages (blocking)
	h.readPump(connection, redisKey)
}

// pingLoop sends Ping messages every 20 seconds
func (h *WebSocketHandler) pingLoop(conn *Connection, redisKey string) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			conn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("ERROR: Failed to send Ping to printer_id %s: %v", conn.printerID, err)
				return
			}
			log.Printf("DEBUG: Ping sent to printer_id: '%s'", conn.printerID)
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection
func (h *WebSocketHandler) writePump(conn *Connection) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.conn.Close()
	}()

	for {
		select {
		case message, ok := <-conn.send:
			log.Printf("DEBUG: writePump received message for printer_id: '%s', length: %d", conn.printerID, len(message))
			conn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				log.Printf("DEBUG: Send channel closed for printer_id: '%s'", conn.printerID)
				conn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("ERROR: Failed to get NextWriter for printer_id %s: %v", conn.printerID, err)
				return
			}
			n, writeErr := w.Write(message)
			if writeErr != nil {
				log.Printf("ERROR: Failed to write message for printer_id %s: %v", conn.printerID, writeErr)
			} else {
				log.Printf("SUCCESS: Wrote %d bytes to WebSocket for printer_id: '%s'", n, conn.printerID)
			}

			// Send queued messages
			queuedCount := len(conn.send)
			for i := 0; i < queuedCount; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-conn.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			// Send ping to keep connection alive
			conn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads messages from the WebSocket connection
func (h *WebSocketHandler) readPump(conn *Connection, redisKey string) {
	defer func() {
		conn.conn.Close()
		// Connection closed - remove Redis key
		ctx := context.Background()
		if err := h.redisClient.Delete(ctx, redisKey); err != nil {
			log.Printf("WARNING: Failed to delete Redis key for printer_id %s: %v", conn.printerID, err)
		} else {
			log.Printf("INFO: Removed Redis key for printer_id: '%s'", conn.printerID)
		}
	}()

	for {
		_, _, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ERROR: WebSocket error for printer_id %s: %v", conn.printerID, err)
			} else {
				log.Printf("INFO: WebSocket connection closed for printer_id: '%s'", conn.printerID)
			}
			break
		}
		// Reset read deadline after receiving any message
		conn.conn.SetReadDeadline(time.Now().Add(25 * time.Second))
	}
}

