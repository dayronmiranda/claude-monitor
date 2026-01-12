package services

import (
	"sync"
	"time"
)

// AnalyticsService maneja estadísticas con cache
type AnalyticsService struct {
	claude        *ClaudeService
	mu            sync.RWMutex
	global        *GlobalAnalytics
	projects      map[string]*ProjectAnalytics
	globalTTL     time.Time
	projectTTL    map[string]time.Time
	cacheDuration time.Duration
}

// GlobalAnalytics estadísticas globales
type GlobalAnalytics struct {
	TotalProjects         int               `json:"total_projects"`
	TotalSessions         int               `json:"total_sessions"`
	TotalMessages         int               `json:"total_messages"`
	TotalUserMessages     int               `json:"total_user_messages"`
	TotalAssistantMessages int              `json:"total_assistant_messages"`
	TotalSizeBytes        int64             `json:"total_size_bytes"`
	EmptySessions         int               `json:"empty_sessions"`
	ActiveDays            int               `json:"active_days"`
	ProjectsSummary       []ProjectSummary  `json:"projects_summary"`
	DailyActivity         []DailyActivity   `json:"daily_activity"`
	LastUpdated           time.Time         `json:"last_updated"`
	CachedUntil           time.Time         `json:"cached_until"`
}

// ProjectSummary resumen de proyecto
type ProjectSummary struct {
	Path              string    `json:"path"`
	RealPath          string    `json:"real_path"`
	Sessions          int       `json:"sessions"`
	Messages          int       `json:"messages"`
	UserMessages      int       `json:"user_messages"`
	AssistantMessages int       `json:"assistant_messages"`
	SizeBytes         int64     `json:"size_bytes"`
	EmptySessions     int       `json:"empty_sessions"`
	LastActivity      time.Time `json:"last_activity"`
}

// ProjectAnalytics estadísticas de un proyecto
type ProjectAnalytics struct {
	Path                  string          `json:"path"`
	RealPath              string          `json:"real_path"`
	TotalSessions         int             `json:"total_sessions"`
	TotalMessages         int             `json:"total_messages"`
	TotalUserMessages     int             `json:"total_user_messages"`
	TotalAssistantMessages int            `json:"total_assistant_messages"`
	TotalSizeBytes        int64           `json:"total_size_bytes"`
	EmptySessions         int             `json:"empty_sessions"`
	DailyActivity         []DailyActivity `json:"daily_activity"`
	TopDays               []DailyActivity `json:"top_days"`
	LastUpdated           time.Time       `json:"last_updated"`
	CachedUntil           time.Time       `json:"cached_until"`
}

// NewAnalyticsService crea una nueva instancia
func NewAnalyticsService(claude *ClaudeService, cacheDuration time.Duration) *AnalyticsService {
	if cacheDuration == 0 {
		cacheDuration = 5 * time.Minute
	}
	return &AnalyticsService{
		claude:        claude,
		projects:      make(map[string]*ProjectAnalytics),
		projectTTL:    make(map[string]time.Time),
		cacheDuration: cacheDuration,
	}
}

// GetGlobal obtiene estadísticas globales
func (s *AnalyticsService) GetGlobal(forceRefresh bool) (*GlobalAnalytics, error) {
	s.mu.RLock()
	if !forceRefresh && s.global != nil && time.Now().Before(s.globalTTL) {
		cached := s.global
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	projects, err := s.claude.ListProjects()
	if err != nil {
		return nil, err
	}

	global := &GlobalAnalytics{
		TotalProjects:   len(projects),
		ProjectsSummary: make([]ProjectSummary, 0),
		LastUpdated:     time.Now(),
	}

	allActivity := make(map[string]*DailyActivity)

	for _, p := range projects {
		sessions, err := s.claude.ListSessions(p.Path)
		if err != nil {
			continue
		}

		summary := ProjectSummary{
			Path:     p.Path,
			RealPath: p.RealPath,
			Sessions: len(sessions),
		}

		for _, sess := range sessions {
			summary.Messages += sess.MessageCount
			summary.UserMessages += sess.UserMessages
			summary.AssistantMessages += sess.AssistantMessages
			summary.SizeBytes += sess.SizeBytes
			if sess.MessageCount == 0 {
				summary.EmptySessions++
			}
			if sess.ModifiedAt.After(summary.LastActivity) {
				summary.LastActivity = sess.ModifiedAt
			}
		}

		global.TotalSessions += summary.Sessions
		global.TotalMessages += summary.Messages
		global.TotalUserMessages += summary.UserMessages
		global.TotalAssistantMessages += summary.AssistantMessages
		global.TotalSizeBytes += summary.SizeBytes
		global.EmptySessions += summary.EmptySessions
		global.ProjectsSummary = append(global.ProjectsSummary, summary)

		activity, _ := s.claude.GetProjectActivity(p.Path)
		for _, a := range activity {
			if _, exists := allActivity[a.Date]; !exists {
				allActivity[a.Date] = &DailyActivity{Date: a.Date}
			}
			allActivity[a.Date].Messages += a.Messages
			allActivity[a.Date].Sessions += a.Sessions
		}
	}

	for _, a := range allActivity {
		global.DailyActivity = append(global.DailyActivity, *a)
	}
	global.ActiveDays = len(allActivity)
	global.CachedUntil = time.Now().Add(s.cacheDuration)

	s.mu.Lock()
	s.global = global
	s.globalTTL = global.CachedUntil
	s.mu.Unlock()

	return global, nil
}

// GetProject obtiene estadísticas de un proyecto
func (s *AnalyticsService) GetProject(projectPath string, forceRefresh bool) (*ProjectAnalytics, error) {
	s.mu.RLock()
	if !forceRefresh {
		if cached, exists := s.projects[projectPath]; exists {
			if ttl, ok := s.projectTTL[projectPath]; ok && time.Now().Before(ttl) {
				s.mu.RUnlock()
				return cached, nil
			}
		}
	}
	s.mu.RUnlock()

	sessions, err := s.claude.ListSessions(projectPath)
	if err != nil {
		return nil, err
	}

	analytics := &ProjectAnalytics{
		Path:          projectPath,
		RealPath:      DecodeProjectPath(projectPath),
		TotalSessions: len(sessions),
		LastUpdated:   time.Now(),
	}

	for _, sess := range sessions {
		analytics.TotalMessages += sess.MessageCount
		analytics.TotalUserMessages += sess.UserMessages
		analytics.TotalAssistantMessages += sess.AssistantMessages
		analytics.TotalSizeBytes += sess.SizeBytes
		if sess.MessageCount == 0 {
			analytics.EmptySessions++
		}
	}

	activity, _ := s.claude.GetProjectActivity(projectPath)
	analytics.DailyActivity = activity

	if len(activity) > 0 {
		sorted := make([]DailyActivity, len(activity))
		copy(sorted, activity)
		// Bubble sort por mensajes (descendente)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].Messages > sorted[i].Messages {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		if len(sorted) > 5 {
			sorted = sorted[:5]
		}
		analytics.TopDays = sorted
	}

	analytics.CachedUntil = time.Now().Add(s.cacheDuration)

	s.mu.Lock()
	s.projects[projectPath] = analytics
	s.projectTTL[projectPath] = analytics.CachedUntil
	s.mu.Unlock()

	return analytics, nil
}

// Invalidate invalida el cache
func (s *AnalyticsService) Invalidate(projectPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectPath == "" {
		s.global = nil
		s.projects = make(map[string]*ProjectAnalytics)
		s.projectTTL = make(map[string]time.Time)
	} else {
		delete(s.projects, projectPath)
		delete(s.projectTTL, projectPath)
		s.global = nil
	}
}

// GetCacheStatus retorna el estado del cache
func (s *AnalyticsService) GetCacheStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"global_cached":    s.global != nil,
		"projects_cached":  len(s.projects),
		"cache_duration":   s.cacheDuration.String(),
	}

	if s.global != nil {
		status["global_expires"] = s.globalTTL
	}

	return status
}
