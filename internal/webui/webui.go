package webui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Entry represents a log entry to be displayed in the UI
type Entry struct {
	Type      string    `json:"type"`      // "log", "user-input", "response"
	Content   string    `json:"content"`   // The actual log message/user input/response
	Timestamp time.Time `json:"timestamp"` // Time of the event
	Channel   string    `json:"channel"`   // Slack channel where the event occurred
	User      string    `json:"user"`      // Slack user ID
}

// WebUI is responsible for serving the web UI and streaming logs
type WebUI struct {
	l           *logrus.Entry
	upgrader    websocket.Upgrader
	clients     map[*websocket.Conn]bool
	clientsMu   sync.Mutex
	broadcastCh chan Entry
	entries     []Entry // Store recent entries to send to new clients
	entriesMu   sync.Mutex
	maxEntries  int
}

// New creates a new WebUI instance
func New(l *logrus.Entry) *WebUI {
	return &WebUI{
		l: l,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all connections for simplicity
				return true
			},
		},
		clients:     make(map[*websocket.Conn]bool),
		broadcastCh: make(chan Entry, 100),
		entries:     make([]Entry, 0, 100),
		maxEntries:  100, // Store last 100 entries
	}
}

// Start starts the web UI server
func (w *WebUI) Start(addr string) error {
	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("internal/webui/static")))

	// WebSocket endpoint
	http.HandleFunc("/ws", w.handleWebSocket)

	// Start the broadcast goroutine
	go w.broadcastMessages()

	// Start the HTTP server
	w.l.Infof("WebUI server started on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// Log logs a generic message
func (w *WebUI) Log(message string) {
	entry := Entry{
		Type:      "log",
		Content:   message,
		Timestamp: time.Now(),
	}
	w.addEntry(entry)
}

// LogUserInput logs a user input message
func (w *WebUI) LogUserInput(user, channel, message string) {
	entry := Entry{
		Type:      "user-input",
		Content:   message,
		Timestamp: time.Now(),
		Channel:   channel,
		User:      user,
	}
	w.addEntry(entry)
}

// LogResponse logs a bot response message
func (w *WebUI) LogResponse(user, channel, message string) {
	entry := Entry{
		Type:      "response",
		Content:   message,
		Timestamp: time.Now(),
		Channel:   channel,
		User:      user,
	}
	w.addEntry(entry)
}

// addEntry adds an entry to the broadcast channel and the entries list
func (w *WebUI) addEntry(entry Entry) {
	// Add to broadcast channel
	select {
	case w.broadcastCh <- entry:
	default:
		w.l.Warn("Broadcast channel full, dropping message")
	}

	// Add to entries list
	w.entriesMu.Lock()
	defer w.entriesMu.Unlock()

	w.entries = append(w.entries, entry)
	if len(w.entries) > w.maxEntries {
		w.entries = w.entries[len(w.entries)-w.maxEntries:]
	}
}

// handleWebSocket handles WebSocket connections
func (w *WebUI) handleWebSocket(rw http.ResponseWriter, req *http.Request) {
	conn, err := w.upgrader.Upgrade(rw, req, nil)
	if err != nil {
		w.l.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	defer conn.Close()

	// Register client
	w.clientsMu.Lock()
	w.clients[conn] = true
	w.clientsMu.Unlock()
	defer func() {
		w.clientsMu.Lock()
		delete(w.clients, conn)
		w.clientsMu.Unlock()
	}()

	// Send recent entries to new client
	w.entriesMu.Lock()
	for _, entry := range w.entries {
		data, err := json.Marshal(entry)
		if err != nil {
			w.l.WithError(err).Error("Failed to marshal log entry")
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			w.l.WithError(err).Error("Failed to send log entry to client")
			break
		}
	}
	w.entriesMu.Unlock()

	// Keep the connection open by reading messages (we don't expect any, but need to satisfy WebSocket protocol)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				w.l.WithError(err).Error("WebSocket read error")
			}
			break
		}
	}
}

// broadcastMessages broadcasts messages to all connected clients
func (w *WebUI) broadcastMessages() {
	for entry := range w.broadcastCh {
		data, err := json.Marshal(entry)
		if err != nil {
			w.l.WithError(err).Error("Failed to marshal log entry")
			continue
		}

		w.clientsMu.Lock()
		for client := range w.clients {
			if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Failed to send log entry to client: %v", err)
				client.Close()
				delete(w.clients, client)
			}
		}
		w.clientsMu.Unlock()
	}
}

// GetWebSocketConnCount returns the number of active WebSocket connections
func (w *WebUI) GetWebSocketConnCount() int {
	w.clientsMu.Lock()
	defer w.clientsMu.Unlock()
	return len(w.clients)
}

// LogrusHook is a logrus hook that sends log entries to the web UI
type LogrusHook struct {
	webUI *WebUI
}

// NewLogrusHook creates a new logrus hook for the web UI
func NewLogrusHook(webUI *WebUI) *LogrusHook {
	return &LogrusHook{
		webUI: webUI,
	}
}

// Fire implements the logrus.Hook interface
func (h *LogrusHook) Fire(entry *logrus.Entry) error {
	// Format the log entry
	formattedEntry := fmt.Sprintf("[%s] %s", entry.Level, entry.Message)

	// Add any fields
	if len(entry.Data) > 0 {
		fields := make([]string, 0, len(entry.Data))
		for k, v := range entry.Data {
			fields = append(fields, fmt.Sprintf("%s=%v", k, v))
		}
		formattedEntry += " " + fmt.Sprintf("%v", fields)
	}

	// Log to web UI
	h.webUI.Log(formattedEntry)
	return nil
}

// Levels implements the logrus.Hook interface
func (h *LogrusHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
