package services

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"claude-monitor/pkg/logger"
)

// WebSocketManager gestiona conexiones WebSocket con cleanup automático
type WebSocketManager struct {
	// Map de terminal ID -> Map de conexiones
	clients    map[string]map[*websocket.Conn]*ClientInfo
	mu         sync.RWMutex
	register   chan *ClientRegistration
	unregister chan *ClientRegistration
	broadcast  chan *BroadcastMessage
	done       chan struct{}

	// Configuración
	pingInterval      time.Duration
	staleTimeout      time.Duration
	cleanupInterval   time.Duration
	maxClientsPerTerm int
}

// ClientInfo información de un cliente WebSocket
type ClientInfo struct {
	Conn         *websocket.Conn
	TerminalID   string
	ConnectedAt  time.Time
	LastActivity time.Time
	UserAgent    string
	RemoteAddr   string
}

// ClientRegistration registro/desregistro de cliente
type ClientRegistration struct {
	TerminalID string
	Conn       *websocket.Conn
	Info       *ClientInfo
	Done       chan error
}

// BroadcastMessage mensaje para broadcast
type BroadcastMessage struct {
	TerminalID string
	Data       interface{}
	Exclude    *websocket.Conn // Excluir esta conexión del broadcast
}

// WebSocketManagerConfig configuración del manager
type WebSocketManagerConfig struct {
	PingInterval      time.Duration
	StaleTimeout      time.Duration
	CleanupInterval   time.Duration
	MaxClientsPerTerm int
}

// DefaultWebSocketConfig configuración por defecto
func DefaultWebSocketConfig() WebSocketManagerConfig {
	return WebSocketManagerConfig{
		PingInterval:      30 * time.Second,
		StaleTimeout:      5 * time.Minute,
		CleanupInterval:   30 * time.Second,
		MaxClientsPerTerm: 10,
	}
}

// NewWebSocketManager crea un nuevo manager
func NewWebSocketManager(cfg WebSocketManagerConfig) *WebSocketManager {
	m := &WebSocketManager{
		clients:           make(map[string]map[*websocket.Conn]*ClientInfo),
		register:          make(chan *ClientRegistration, 100),
		unregister:        make(chan *ClientRegistration, 100),
		broadcast:         make(chan *BroadcastMessage, 1000),
		done:              make(chan struct{}),
		pingInterval:      cfg.PingInterval,
		staleTimeout:      cfg.StaleTimeout,
		cleanupInterval:   cfg.CleanupInterval,
		maxClientsPerTerm: cfg.MaxClientsPerTerm,
	}

	go m.run()
	return m
}

// run loop principal del manager
func (m *WebSocketManager) run() {
	cleanupTicker := time.NewTicker(m.cleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case reg := <-m.register:
			m.addClient(reg)

		case reg := <-m.unregister:
			m.removeClient(reg)

		case msg := <-m.broadcast:
			m.broadcastToTerminal(msg)

		case <-cleanupTicker.C:
			m.cleanupStaleConnections()

		case <-m.done:
			m.closeAllConnections()
			return
		}
	}
}

// Register registra un nuevo cliente
func (m *WebSocketManager) Register(terminalID string, conn *websocket.Conn, userAgent, remoteAddr string) error {
	done := make(chan error, 1)

	m.register <- &ClientRegistration{
		TerminalID: terminalID,
		Conn:       conn,
		Info: &ClientInfo{
			Conn:         conn,
			TerminalID:   terminalID,
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
			UserAgent:    userAgent,
			RemoteAddr:   remoteAddr,
		},
		Done: done,
	}

	return <-done
}

// Unregister desregistra un cliente
func (m *WebSocketManager) Unregister(terminalID string, conn *websocket.Conn) {
	m.unregister <- &ClientRegistration{
		TerminalID: terminalID,
		Conn:       conn,
	}
}

// Broadcast envía mensaje a todos los clientes de un terminal
func (m *WebSocketManager) Broadcast(terminalID string, data interface{}) {
	m.broadcast <- &BroadcastMessage{
		TerminalID: terminalID,
		Data:       data,
	}
}

// BroadcastExcept envía mensaje excluyendo una conexión
func (m *WebSocketManager) BroadcastExcept(terminalID string, data interface{}, exclude *websocket.Conn) {
	m.broadcast <- &BroadcastMessage{
		TerminalID: terminalID,
		Data:       data,
		Exclude:    exclude,
	}
}

// UpdateActivity actualiza timestamp de actividad
func (m *WebSocketManager) UpdateActivity(terminalID string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clients, ok := m.clients[terminalID]; ok {
		if info, ok := clients[conn]; ok {
			info.LastActivity = time.Now()
		}
	}
}

// GetClientCount retorna número de clientes para un terminal
func (m *WebSocketManager) GetClientCount(terminalID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if clients, ok := m.clients[terminalID]; ok {
		return len(clients)
	}
	return 0
}

// GetTotalClientCount retorna número total de clientes
func (m *WebSocketManager) GetTotalClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, clients := range m.clients {
		total += len(clients)
	}
	return total
}

// GetClients retorna info de clientes para un terminal
func (m *WebSocketManager) GetClients(terminalID string) []*ClientInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ClientInfo
	if clients, ok := m.clients[terminalID]; ok {
		for _, info := range clients {
			// Copiar para evitar race conditions
			infoCopy := *info
			result = append(result, &infoCopy)
		}
	}
	return result
}

// Shutdown cierra el manager ordenadamente
func (m *WebSocketManager) Shutdown() {
	close(m.done)
}

// addClient agrega un cliente
func (m *WebSocketManager) addClient(reg *ClientRegistration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verificar límite de clientes
	if clients, ok := m.clients[reg.TerminalID]; ok {
		if len(clients) >= m.maxClientsPerTerm {
			reg.Done <- &maxClientsError{terminalID: reg.TerminalID}
			return
		}
	} else {
		m.clients[reg.TerminalID] = make(map[*websocket.Conn]*ClientInfo)
	}

	m.clients[reg.TerminalID][reg.Conn] = reg.Info

	logger.Debug("WebSocket client registered",
		"terminal", reg.TerminalID,
		"remote_addr", reg.Info.RemoteAddr,
		"total_clients", len(m.clients[reg.TerminalID]))

	reg.Done <- nil
}

// removeClient elimina un cliente
func (m *WebSocketManager) removeClient(reg *ClientRegistration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clients, ok := m.clients[reg.TerminalID]; ok {
		if info, ok := clients[reg.Conn]; ok {
			logger.Debug("WebSocket client unregistered",
				"terminal", reg.TerminalID,
				"remote_addr", info.RemoteAddr,
				"duration", time.Since(info.ConnectedAt))

			delete(clients, reg.Conn)

			// Limpiar map si está vacío
			if len(clients) == 0 {
				delete(m.clients, reg.TerminalID)
			}
		}
	}
}

// broadcastToTerminal envía mensaje a clientes de un terminal
func (m *WebSocketManager) broadcastToTerminal(msg *BroadcastMessage) {
	m.mu.RLock()
	clients, ok := m.clients[msg.TerminalID]
	if !ok {
		m.mu.RUnlock()
		return
	}

	// Copiar slice de conexiones para evitar lock mientras enviamos
	var conns []*websocket.Conn
	for conn := range clients {
		if conn != msg.Exclude {
			conns = append(conns, conn)
		}
	}
	m.mu.RUnlock()

	// Enviar a cada conexión
	for _, conn := range conns {
		if err := conn.WriteJSON(msg.Data); err != nil {
			// Marcar para cleanup
			m.Unregister(msg.TerminalID, conn)
		}
	}
}

// cleanupStaleConnections limpia conexiones inactivas
func (m *WebSocketManager) cleanupStaleConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	threshold := time.Now().Add(-m.staleTimeout)
	cleaned := 0

	for termID, clients := range m.clients {
		for conn, info := range clients {
			if info.LastActivity.Before(threshold) {
				// Intentar ping antes de cerrar
				if err := conn.WriteControl(
					websocket.PingMessage,
					nil,
					time.Now().Add(time.Second),
				); err != nil {
					// No responde, cerrar conexión
					conn.Close()
					delete(clients, conn)
					cleaned++

					logger.Debug("Cleaned stale WebSocket connection",
						"terminal", termID,
						"remote_addr", info.RemoteAddr,
						"last_activity", info.LastActivity)
				} else {
					// Responde al ping, actualizar actividad
					info.LastActivity = time.Now()
				}
			}
		}

		// Limpiar map si está vacío
		if len(clients) == 0 {
			delete(m.clients, termID)
		}
	}

	if cleaned > 0 {
		logger.Info("Cleaned stale WebSocket connections", "count", cleaned)
	}
}

// closeAllConnections cierra todas las conexiones
func (m *WebSocketManager) closeAllConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for termID, clients := range m.clients {
		for conn, info := range clients {
			// Enviar mensaje de cierre
			conn.WriteJSON(map[string]string{
				"type":    "shutdown",
				"message": "Server shutting down",
			})
			conn.Close()

			logger.Debug("Closed WebSocket connection on shutdown",
				"terminal", termID,
				"remote_addr", info.RemoteAddr)
		}
	}

	m.clients = make(map[string]map[*websocket.Conn]*ClientInfo)
	logger.Info("All WebSocket connections closed")
}

// maxClientsError error cuando se alcanza el límite de clientes
type maxClientsError struct {
	terminalID string
}

func (e *maxClientsError) Error() string {
	return "max clients reached for terminal: " + e.terminalID
}
