package websocket

import (
	"log"
	"sync"
	
	"github.com/gorilla/websocket"
)

// Hub manages WebSocket connections by printer_id
type Hub struct {
	// Map of printer_id -> WebSocket connection
	connections sync.Map // map[string]*Connection
}

// Connection represents a WebSocket connection with metadata
type Connection struct {
	conn      *websocket.Conn
	printerID string
	send      chan []byte
	hub       *Hub
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		connections: sync.Map{},
	}
}

// Register adds a connection to the hub
func (h *Hub) Register(printerID string, conn *websocket.Conn) *Connection {
	c := &Connection{
		conn:      conn,
		printerID: printerID,
		send:      make(chan []byte, 256),
		hub:       h,
	}
	
	h.connections.Store(printerID, c)
	log.Printf("DEBUG: Connection registered for printer_id: '%s'", printerID)
	return c
}

// Unregister removes a connection from the hub
func (h *Hub) Unregister(printerID string) {
	if conn, ok := h.connections.LoadAndDelete(printerID); ok {
		close(conn.(*Connection).send)
		log.Printf("DEBUG: Connection unregistered for printer_id: '%s'", printerID)
	}
}

// GetConnection retrieves a connection by printer_id
func (h *Hub) GetConnection(printerID string) (*Connection, bool) {
	conn, ok := h.connections.Load(printerID)
	if !ok {
		return nil, false
	}
	return conn.(*Connection), true
}

// SendToPrinter sends a message to a specific printer
func (h *Hub) SendToPrinter(printerID string, message []byte) error {
	log.Printf("DEBUG: SendToPrinter called - printer_id: '%s', message length: %d", printerID, len(message))
	
	conn, ok := h.GetConnection(printerID)
	if !ok {
		log.Printf("ERROR: Printer '%s' not found in connections map", printerID)
		// List all connected printer_ids for debugging
		h.connections.Range(func(key, value interface{}) bool {
			log.Printf("DEBUG: Connected printer_id: '%s'", key)
			return true
		})
		return ErrPrinterNotConnected
	}
	
	log.Printf("DEBUG: Found connection for printer_id: '%s', sending message to channel", printerID)
	
	select {
	case conn.send <- message:
		log.Printf("SUCCESS: Message sent to channel for printer_id: '%s'", printerID)
		return nil
	default:
		log.Printf("ERROR: Send buffer full for printer_id: '%s'", printerID)
		return ErrSendBufferFull
	}
}

// ListConnectedPrinters returns a list of all connected printer IDs (for debugging)
func (h *Hub) ListConnectedPrinters() []string {
	var printerIDs []string
	h.connections.Range(func(key, value interface{}) bool {
		if printerID, ok := key.(string); ok {
			printerIDs = append(printerIDs, printerID)
		}
		return true
	})
	return printerIDs
}

