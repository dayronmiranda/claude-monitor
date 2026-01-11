package services

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ClaudeService maneja operaciones con proyectos y sesiones de Claude
type ClaudeService struct {
	claudeDir string
}

// ClaudeProject representa un proyecto de Claude
type ClaudeProject struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	RealPath     string    `json:"real_path"`
	SessionCount int       `json:"session_count"`
	LastModified time.Time `json:"last_modified"`
}

// ClaudeSession representa una sesión de Claude
type ClaudeSession struct {
	ID           string    `json:"id"`
	Name         string    `json:"name,omitempty"`
	ProjectPath  string    `json:"project_path"`
	RealPath     string    `json:"real_path"`
	FilePath     string    `json:"file_path"`
	FirstMessage string    `json:"first_message"`
	MessageCount int       `json:"message_count"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	ModifiedAt   time.Time `json:"modified_at"`
}

// SessionNames almacena nombres personalizados de sesiones
type SessionNames struct {
	Names map[string]string `json:"names"` // sessionID -> name
}

var sessionNames = &SessionNames{Names: make(map[string]string)}
var sessionNamesFile = ""

// InitSessionNames carga los nombres de sesiones desde archivo
func InitSessionNames(dataDir string) error {
	sessionNamesFile = filepath.Join(dataDir, "session_names.json")

	data, err := os.ReadFile(sessionNamesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No hay archivo, usar mapa vacío
		}
		return err
	}

	return json.Unmarshal(data, sessionNames)
}

// saveSessionNames guarda los nombres de sesiones a archivo
func saveSessionNames() error {
	if sessionNamesFile == "" {
		return nil
	}

	data, err := json.MarshalIndent(sessionNames, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionNamesFile, data, 0600)
}

// GetSessionName obtiene el nombre personalizado de una sesión
func GetSessionName(sessionID string) string {
	if name, ok := sessionNames.Names[sessionID]; ok {
		return name
	}
	return ""
}

// SetSessionName establece el nombre personalizado de una sesión
func SetSessionName(sessionID, name string) error {
	if name == "" {
		delete(sessionNames.Names, sessionID)
	} else {
		sessionNames.Names[sessionID] = name
	}
	return saveSessionNames()
}

// SessionMessage representa un mensaje individual de una sesión
type SessionMessage struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// DailyActivity actividad diaria
type DailyActivity struct {
	Date     string `json:"date"`
	Messages int    `json:"messages"`
	Sessions int    `json:"sessions"`
}

// NewClaudeService crea una nueva instancia del servicio
func NewClaudeService(claudeDir string) *ClaudeService {
	if claudeDir == "" {
		home, _ := os.UserHomeDir()
		claudeDir = filepath.Join(home, ".claude", "projects")
	}
	return &ClaudeService{claudeDir: claudeDir}
}

// GetClaudeDir retorna el directorio de Claude
func (s *ClaudeService) GetClaudeDir() string {
	return s.claudeDir
}

// DecodeProjectPath decodifica un path de proyecto (fallback simple)
func DecodeProjectPath(encoded string) string {
	if encoded == "-" {
		return "/"
	}
	decoded := strings.TrimPrefix(encoded, "-")
	decoded = strings.ReplaceAll(decoded, "-", "/")
	return "/" + decoded
}

// GetRealPathFromSessions extrae el cwd real de los archivos de sesión
func (s *ClaudeService) GetRealPathFromSessions(projectPath string) string {
	fullPath := filepath.Join(s.claudeDir, projectPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return DecodeProjectPath(projectPath)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isValidUUIDSession(entry.Name()) {
			continue
		}

		filePath := filepath.Join(fullPath, entry.Name())
		if cwd := extractCwdFromSession(filePath); cwd != "" {
			return cwd
		}
	}

	return DecodeProjectPath(projectPath)
}

// extractCwdFromSession extrae el cwd del primer mensaje de una sesión
func extractCwdFromSession(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if cwd, ok := msg["cwd"].(string); ok && cwd != "" {
			return cwd
		}
	}

	return ""
}

// EncodeProjectPath codifica un path real
func EncodeProjectPath(realPath string) string {
	if realPath == "/" {
		return "-"
	}
	encoded := strings.TrimPrefix(realPath, "/")
	encoded = strings.ReplaceAll(encoded, "/", "-")
	return "-" + encoded
}

// isValidUUIDSession verifica si un nombre de archivo es una sesión válida
func isValidUUIDSession(name string) bool {
	if strings.HasPrefix(name, "agent-") {
		return false
	}
	pattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\.jsonl$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

// extractSessionID extrae el ID de un nombre de archivo
func extractSessionID(filename string) string {
	return strings.TrimSuffix(filename, ".jsonl")
}

// ListProjects lista todos los proyectos de Claude
func (s *ClaudeService) ListProjects() ([]ClaudeProject, error) {
	entries, err := os.ReadDir(s.claudeDir)
	if err != nil {
		return nil, err
	}

	var projects []ClaudeProject
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "." {
			continue
		}

		projectPath := filepath.Join(s.claudeDir, entry.Name())
		info, _ := entry.Info()

		sessionCount := 0
		sessionFiles, _ := os.ReadDir(projectPath)
		for _, sf := range sessionFiles {
			if isValidUUIDSession(sf.Name()) {
				sessionCount++
			}
		}

		var lastMod time.Time
		if info != nil {
			lastMod = info.ModTime()
		}

		projects = append(projects, ClaudeProject{
			ID:           entry.Name(),
			Path:         entry.Name(),
			RealPath:     s.GetRealPathFromSessions(entry.Name()),
			SessionCount: sessionCount,
			LastModified: lastMod,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

// GetProject obtiene un proyecto específico
func (s *ClaudeService) GetProject(projectPath string) (*ClaudeProject, error) {
	fullPath := filepath.Join(s.claudeDir, projectPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	sessionCount := 0
	sessionFiles, _ := os.ReadDir(fullPath)
	for _, sf := range sessionFiles {
		if isValidUUIDSession(sf.Name()) {
			sessionCount++
		}
	}

	return &ClaudeProject{
		ID:           projectPath,
		Path:         projectPath,
		RealPath:     s.GetRealPathFromSessions(projectPath),
		SessionCount: sessionCount,
		LastModified: info.ModTime(),
	}, nil
}

// DeleteProject elimina un proyecto completo
func (s *ClaudeService) DeleteProject(projectPath string) error {
	fullPath := filepath.Join(s.claudeDir, projectPath)
	return os.RemoveAll(fullPath)
}

// ListSessions lista las sesiones de un proyecto
func (s *ClaudeService) ListSessions(projectPath string) ([]ClaudeSession, error) {
	fullPath := filepath.Join(s.claudeDir, projectPath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var sessions []ClaudeSession
	for _, entry := range entries {
		if entry.IsDir() || !isValidUUIDSession(entry.Name()) {
			continue
		}

		sessionID := extractSessionID(entry.Name())
		filePath := filepath.Join(fullPath, entry.Name())
		info, _ := entry.Info()

		// Extraer el cwd real de la sesión
		realPath := extractCwdFromSession(filePath)
		if realPath == "" {
			realPath = DecodeProjectPath(projectPath)
		}

		session := ClaudeSession{
			ID:          sessionID,
			ProjectPath: projectPath,
			RealPath:    realPath,
			FilePath:    filePath,
		}

		if info != nil {
			session.ModifiedAt = info.ModTime()
			session.SizeBytes = info.Size()
		}

		firstMsg, msgCount, createdAt := s.parseSessionFile(filePath)
		session.FirstMessage = firstMsg
		session.MessageCount = msgCount
		session.CreatedAt = createdAt
		session.Name = GetSessionName(session.ID)

		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModifiedAt.After(sessions[j].ModifiedAt)
	})

	return sessions, nil
}

// GetSession obtiene una sesión específica
func (s *ClaudeService) GetSession(projectPath, sessionID string) (*ClaudeSession, error) {
	filePath := filepath.Join(s.claudeDir, projectPath, sessionID+".jsonl")

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// Extraer el cwd real de la sesión
	realPath := extractCwdFromSession(filePath)
	if realPath == "" {
		realPath = DecodeProjectPath(projectPath)
	}

	session := &ClaudeSession{
		ID:          sessionID,
		ProjectPath: projectPath,
		RealPath:    realPath,
		FilePath:    filePath,
		ModifiedAt:  info.ModTime(),
		SizeBytes:   info.Size(),
	}

	firstMsg, msgCount, createdAt := s.parseSessionFile(filePath)
	session.FirstMessage = firstMsg
	session.MessageCount = msgCount
	session.CreatedAt = createdAt

	return session, nil
}

// DeleteSession elimina una sesión
func (s *ClaudeService) DeleteSession(projectPath, sessionID string) error {
	filePath := filepath.Join(s.claudeDir, projectPath, sessionID+".jsonl")

	// Eliminar directorio de subagentes si existe
	subagentsDir := filepath.Join(s.claudeDir, projectPath, sessionID, "subagents")
	os.RemoveAll(subagentsDir)
	os.Remove(filepath.Join(s.claudeDir, projectPath, sessionID))

	return os.Remove(filePath)
}

// DeleteMultipleSessions elimina múltiples sesiones
func (s *ClaudeService) DeleteMultipleSessions(projectPath string, sessionIDs []string) (int, error) {
	deleted := 0
	for _, id := range sessionIDs {
		if err := s.DeleteSession(projectPath, id); err == nil {
			deleted++
		}
	}
	return deleted, nil
}

// CleanEmptySessions elimina sesiones con 0 mensajes
func (s *ClaudeService) CleanEmptySessions(projectPath string) (int, error) {
	sessions, err := s.ListSessions(projectPath)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, sess := range sessions {
		if sess.MessageCount == 0 {
			if err := s.DeleteSession(projectPath, sess.ID); err == nil {
				deleted++
			}
		}
	}
	return deleted, nil
}

// GetProjectActivity obtiene la actividad diaria de un proyecto
func (s *ClaudeService) GetProjectActivity(projectPath string) ([]DailyActivity, error) {
	fullPath := filepath.Join(s.claudeDir, projectPath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	activityMap := make(map[string]*DailyActivity)
	sessionDates := make(map[string]map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || !isValidUUIDSession(entry.Name()) {
			continue
		}

		sessionID := extractSessionID(entry.Name())
		filePath := filepath.Join(fullPath, entry.Name())

		dates := s.parseSessionDates(filePath)
		for date, count := range dates {
			if _, exists := activityMap[date]; !exists {
				activityMap[date] = &DailyActivity{Date: date, Messages: 0, Sessions: 0}
				sessionDates[date] = make(map[string]bool)
			}
			activityMap[date].Messages += count
			sessionDates[date][sessionID] = true
		}
	}

	for date, sessions := range sessionDates {
		activityMap[date].Sessions = len(sessions)
	}

	var activities []DailyActivity
	for _, activity := range activityMap {
		activities = append(activities, *activity)
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Date < activities[j].Date
	})

	return activities, nil
}

// parseSessionFile extrae información de un archivo de sesión
func (s *ClaudeService) parseSessionFile(filePath string) (firstMessage string, messageCount int, createdAt time.Time) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, time.Time{}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msgType, ok := msg["type"].(string); ok && msgType == "user" {
			messageCount++

			if ts, ok := msg["timestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					if createdAt.IsZero() {
						createdAt = t
					}
				}
			}

			if firstMessage == "" {
				if message, ok := msg["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok {
						firstMessage = content
						if len(firstMessage) > 100 {
							firstMessage = firstMessage[:100] + "..."
						}
					}
				}
			}
		}

		if msgType, ok := msg["type"].(string); ok && msgType == "assistant" {
			messageCount++
		}
	}

	return firstMessage, messageCount, createdAt
}

// parseSessionDates extrae las fechas de mensajes
func (s *ClaudeService) parseSessionDates(filePath string) map[string]int {
	dates := make(map[string]int)

	file, err := os.Open(filePath)
	if err != nil {
		return dates
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msgType, ok := msg["type"].(string); ok && msgType == "user" {
			if ts, ok := msg["timestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					date := t.Format("2006-01-02")
					dates[date]++
				}
			}
		}
	}

	return dates
}

// GetSessionMessages obtiene todos los mensajes de una sesión
func (s *ClaudeService) GetSessionMessages(projectPath, sessionID string) ([]SessionMessage, error) {
	filePath := filepath.Join(s.claudeDir, projectPath, sessionID+".jsonl")

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var messages []SessionMessage
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok || (msgType != "user" && msgType != "assistant") {
			continue
		}

		var content string
		if message, ok := msg["message"].(map[string]interface{}); ok {
			if c, ok := message["content"].(string); ok {
				content = c
			}
		}

		var timestamp time.Time
		if ts, ok := msg["timestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				timestamp = t
			}
		}

		messages = append(messages, SessionMessage{
			Type:      msgType,
			Content:   content,
			Timestamp: timestamp,
		})
	}

	return messages, nil
}
