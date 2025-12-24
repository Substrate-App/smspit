// SMSpit - The Mailpit of SMS Testing
// A modern, self-hosted SMS testing server for development
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	_ "modernc.org/sqlite"
)

//go:embed static/*
var staticFiles embed.FS

// Config holds application configuration
type Config struct {
	DBPath        string
	WebPort       string
	APIPort       string
	MaxMessages   int
	TwilioCompat  bool
	AuthToken     string
	CORSOrigins   string
}

// Message represents a captured SMS message
type Message struct {
	ID        string    `json:"id"`
	To        string    `json:"to"`
	From      string    `json:"from,omitempty"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// SendRequest represents an incoming SMS send request
type SendRequest struct {
	To   string   `json:"to"`
	From string   `json:"from,omitempty"`
	Body string   `json:"body"`
	Tags []string `json:"tags,omitempty"`
	// Twilio compatibility fields
	Message string `json:"Message,omitempty"` // Twilio uses "Message" not "body"
}

// Server holds the application state
type Server struct {
	config     Config
	messages   []Message
	mu         sync.RWMutex
	wsClients  map[*websocket.Conn]bool
	wsMu       sync.Mutex
	upgrader   websocket.Upgrader
}

// NewServer creates a new SMSpit server
func NewServer(config Config) *Server {
	return &Server{
		config:    config,
		messages:  make([]Message, 0),
		wsClients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local dev
			},
		},
	}
}

// Middleware for CORS
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.config.CORSOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Middleware for optional auth
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.AuthToken != "" {
			token := r.Header.Get("Authorization")
			if token != "Bearer "+s.config.AuthToken && token != s.config.AuthToken {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// handleSend captures an SMS message
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Handle Twilio compatibility
	body := req.Body
	if body == "" && req.Message != "" {
		body = req.Message
	}

	if req.To == "" {
		http.Error(w, "Missing 'to' field", http.StatusBadRequest)
		return
	}
	if body == "" {
		http.Error(w, "Missing 'body' field", http.StatusBadRequest)
		return
	}

	msg := Message{
		ID:        "msg_" + uuid.New().String()[:8],
		To:        req.To,
		From:      req.From,
		Body:      body,
		Tags:      req.Tags,
		Status:    "captured",
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.messages = append([]Message{msg}, s.messages...) // Prepend (newest first)
	
	// Enforce max messages limit
	if len(s.messages) > s.config.MaxMessages {
		s.messages = s.messages[:s.config.MaxMessages]
	}
	s.mu.Unlock()

	// Broadcast to WebSocket clients
	s.broadcastMessage(msg)

	log.Printf("📱 SMS captured: To=%s Body=%s", msg.To, truncate(msg.Body, 50))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        msg.ID,
		"status":    "captured",
		"timestamp": msg.CreatedAt,
	})
}

// handleTwilioSend handles Twilio-compatible requests
func (s *Server) handleTwilioSend(w http.ResponseWriter, r *http.Request) {
	// Twilio sends form-encoded data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	to := r.FormValue("To")
	from := r.FormValue("From")
	body := r.FormValue("Body")

	if to == "" || body == "" {
		http.Error(w, "Missing To or Body", http.StatusBadRequest)
		return
	}

	msg := Message{
		ID:        "SM" + uuid.New().String()[:32], // Twilio-style ID
		To:        to,
		From:      from,
		Body:      body,
		Status:    "captured",
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.messages = append([]Message{msg}, s.messages...)
	if len(s.messages) > s.config.MaxMessages {
		s.messages = s.messages[:s.config.MaxMessages]
	}
	s.mu.Unlock()

	s.broadcastMessage(msg)

	log.Printf("📱 SMS captured (Twilio): To=%s Body=%s", msg.To, truncate(msg.Body, 50))

	// Return Twilio-compatible response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sid":          msg.ID,
		"status":       "queued",
		"to":           msg.To,
		"from":         msg.From,
		"body":         msg.Body,
		"date_created": msg.CreatedAt.Format(time.RFC3339),
	})
}

// handleListMessages returns all captured messages
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": s.messages,
		"total":    len(s.messages),
	})
}

// handleSearchMessages searches messages
func (s *Server) handleSearchMessages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	to := r.URL.Query().Get("to")

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Message
	for _, msg := range s.messages {
		match := true
		if query != "" && !contains(msg.Body, query) && !contains(msg.To, query) {
			match = false
		}
		if to != "" && !contains(msg.To, to) {
			match = false
		}
		if match {
			results = append(results, msg)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": results,
		"total":    len(results),
	})
}

// handleGetMessage returns a single message by ID
func (s *Server) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, msg := range s.messages {
		if msg.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(msg)
			return
		}
	}

	http.Error(w, "Message not found", http.StatusNotFound)
}

// handleDeleteMessages clears all messages
func (s *Server) handleDeleteMessages(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.messages = make([]Message, 0)
	s.mu.Unlock()

	log.Printf("🗑️ All messages cleared")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

// handleDeleteMessage deletes a single message
func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, msg := range s.messages {
		if msg.ID == id {
			s.messages = append(s.messages[:i], s.messages[i+1:]...)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
			return
		}
	}

	http.Error(w, "Message not found", http.StatusNotFound)
}

// handleWebSocket handles WebSocket connections for real-time updates
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()

	log.Printf("🔌 WebSocket client connected")

	// Keep connection alive and handle disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.wsMu.Lock()
			delete(s.wsClients, conn)
			s.wsMu.Unlock()
			conn.Close()
			log.Printf("🔌 WebSocket client disconnected")
			break
		}
	}
}

// broadcastMessage sends a message to all WebSocket clients
func (s *Server) broadcastMessage(msg Message) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	data, _ := json.Marshal(map[string]interface{}{
		"type":    "new_message",
		"message": msg,
	})

	for client := range s.wsClients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			client.Close()
			delete(s.wsClients, client)
		}
	}
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	count := len(s.messages)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "healthy",
		"message_count": count,
		"version":       "1.0.0",
	})
}

// handleStats returns server statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Calculate stats
	phoneNumbers := make(map[string]int)
	var last24h, lastHour int
	now := time.Now()

	for _, msg := range s.messages {
		phoneNumbers[msg.To]++
		if now.Sub(msg.CreatedAt) < 24*time.Hour {
			last24h++
		}
		if now.Sub(msg.CreatedAt) < time.Hour {
			lastHour++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_messages":      len(s.messages),
		"unique_recipients":   len(phoneNumbers),
		"messages_last_24h":   last24h,
		"messages_last_hour":  lastHour,
		"websocket_clients":   len(s.wsClients),
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var i int
		fmt.Sscanf(val, "%d", &i)
		return i
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}

func main() {
	config := Config{
		DBPath:       getEnv("SMSPIT_DB_PATH", "./smspit.db"),
		WebPort:      getEnv("SMSPIT_WEB_PORT", "8080"),
		APIPort:      getEnv("SMSPIT_API_PORT", "9080"),
		MaxMessages:  getEnvInt("SMSPIT_MAX_MESSAGES", 10000),
		TwilioCompat: getEnvBool("SMSPIT_TWILIO_COMPAT", false),
		AuthToken:    getEnv("SMSPIT_AUTH_TOKEN", ""),
		CORSOrigins:  getEnv("SMSPIT_CORS_ORIGINS", "*"),
	}

	server := NewServer(config)

	// API Router (webhook endpoint)
	apiRouter := mux.NewRouter()
	apiRouter.Use(server.corsMiddleware)
	
	// Main send endpoint
	apiRouter.HandleFunc("/send", server.handleSend).Methods("POST", "OPTIONS")
	apiRouter.HandleFunc("/health", server.handleHealth).Methods("GET")
	
	// Twilio-compatible endpoint
	if config.TwilioCompat {
		apiRouter.HandleFunc("/2010-04-01/Accounts/{accountSid}/Messages.json", server.handleTwilioSend).Methods("POST")
		log.Printf("📱 Twilio compatibility mode enabled")
	}

	// Web Router (UI + API)
	webRouter := mux.NewRouter()
	webRouter.Use(server.corsMiddleware)
	
	// API endpoints
	api := webRouter.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/messages", server.handleListMessages).Methods("GET")
	api.HandleFunc("/messages/search", server.handleSearchMessages).Methods("GET")
	api.HandleFunc("/messages/{id}", server.handleGetMessage).Methods("GET")
	api.HandleFunc("/messages", server.handleDeleteMessages).Methods("DELETE")
	api.HandleFunc("/messages/{id}", server.handleDeleteMessage).Methods("DELETE")
	api.HandleFunc("/stats", server.handleStats).Methods("GET")
	api.HandleFunc("/health", server.handleHealth).Methods("GET")
	
	// WebSocket
	webRouter.HandleFunc("/ws", server.handleWebSocket)
	
	// Static files (UI)
	staticFS, _ := fs.Sub(staticFiles, "static")
	webRouter.PathPrefix("/").Handler(http.FileServer(http.FS(staticFS)))

	// Start servers
	apiServer := &http.Server{
		Addr:    ":" + config.APIPort,
		Handler: apiRouter,
	}

	webServer := &http.Server{
		Addr:    ":" + config.WebPort,
		Handler: webRouter,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("🚀 SMSpit API server starting on port %s", config.APIPort)
		log.Printf("   POST http://localhost:%s/send - Capture SMS", config.APIPort)
		if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	go func() {
		log.Printf("🌐 SMSpit Web UI starting on port %s", config.WebPort)
		log.Printf("   Open http://localhost:%s in your browser", config.WebPort)
		if err := webServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Web server error: %v", err)
		}
	}()

	log.Printf("📱 SMSpit is ready to capture SMS messages!")

	<-stop

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	apiServer.Shutdown(ctx)
	webServer.Shutdown(ctx)
}

