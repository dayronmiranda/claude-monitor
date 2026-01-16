package patterns

import (
	"regexp"
	"sync"
)

// PatternCache caché de patrones regex compilados
type PatternCache struct {
	patterns map[string]*regexp.Regexp
	mu       sync.RWMutex
}

// Global pattern cache
var (
	globalCache     *PatternCache
	globalCacheOnce sync.Once
)

// ClaudePatterns patrones predefinidos para detección de estado de Claude
var ClaudePatterns = []string{
	// Prompts de permiso
	`(?i)Allow\s+\w+.*to`,
	`\[y/n\]`,
	`\[Y/n\]`,
	`\[y/N\]`,

	// Estados de herramientas
	`(?i)^Running:`,
	`(?i)^Writing:`,
	`(?i)^Reading:`,
	`(?i)^Searching:`,
	`(?i)^Editing:`,

	// Spinners
	`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷]`,
	`\[\d+/\d+\]`,
	`\d+%`,

	// Prompts
	`^>\s*$`,
	`claude>\s*$`,

	// Modos
	`(?i)vim mode`,
	`(?i)plan mode`,
	`-- INSERT --`,
	`-- NORMAL --`,
	`-- VISUAL --`,

	// Errores
	`(?i)^Error:`,
	`(?i)^Warning:`,
	`✓`,
	`✗`,

	// Slash commands
	`^/\w+`,

	// Tokens/costo
	`(?i)tokens?:`,
	`(?i)\$[\d.]+`,

	// Background tasks
	`(?i)background|task \d+`,

	// Checkpoint
	`(?i)checkpoint|rewind`,
}

// New crea un nuevo PatternCache
func New() *PatternCache {
	return &PatternCache{
		patterns: make(map[string]*regexp.Regexp),
	}
}

// NewWithPatterns crea un PatternCache y pre-compila los patrones dados
func NewWithPatterns(patterns []string) *PatternCache {
	pc := New()
	for _, p := range patterns {
		pc.Compile(p)
	}
	return pc
}

// GetGlobal retorna el caché global de patrones
func GetGlobal() *PatternCache {
	globalCacheOnce.Do(func() {
		globalCache = NewWithPatterns(ClaudePatterns)
	})
	return globalCache
}

// Compile compila un patrón y lo almacena en el caché
func (pc *PatternCache) Compile(pattern string) (*regexp.Regexp, error) {
	// Verificar si ya está compilado
	pc.mu.RLock()
	if re, exists := pc.patterns[pattern]; exists {
		pc.mu.RUnlock()
		return re, nil
	}
	pc.mu.RUnlock()

	// Compilar el patrón
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	// Almacenar en caché
	pc.mu.Lock()
	pc.patterns[pattern] = re
	pc.mu.Unlock()

	return re, nil
}

// MustCompile compila un patrón o entra en panic si hay error
func (pc *PatternCache) MustCompile(pattern string) *regexp.Regexp {
	re, err := pc.Compile(pattern)
	if err != nil {
		panic("patterns: Compile(" + pattern + "): " + err.Error())
	}
	return re
}

// Get obtiene un patrón compilado del caché
func (pc *PatternCache) Get(pattern string) (*regexp.Regexp, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	re, exists := pc.patterns[pattern]
	return re, exists
}

// Match verifica si el texto coincide con el patrón
func (pc *PatternCache) Match(pattern, text string) bool {
	re, exists := pc.Get(pattern)
	if !exists {
		// Intentar compilar on-demand
		var err error
		re, err = pc.Compile(pattern)
		if err != nil {
			return false
		}
	}
	return re.MatchString(text)
}

// FindString busca la primera coincidencia del patrón en el texto
func (pc *PatternCache) FindString(pattern, text string) string {
	re, exists := pc.Get(pattern)
	if !exists {
		var err error
		re, err = pc.Compile(pattern)
		if err != nil {
			return ""
		}
	}
	return re.FindString(text)
}

// FindStringSubmatch busca la primera coincidencia con grupos
func (pc *PatternCache) FindStringSubmatch(pattern, text string) []string {
	re, exists := pc.Get(pattern)
	if !exists {
		var err error
		re, err = pc.Compile(pattern)
		if err != nil {
			return nil
		}
	}
	return re.FindStringSubmatch(text)
}

// Size retorna el número de patrones en el caché
func (pc *PatternCache) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.patterns)
}

// Patterns retorna la lista de patrones en el caché
func (pc *PatternCache) Patterns() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	patterns := make([]string, 0, len(pc.patterns))
	for p := range pc.patterns {
		patterns = append(patterns, p)
	}
	return patterns
}

// Funciones helper globales

// Match usa el caché global para verificar coincidencia
func Match(pattern, text string) bool {
	return GetGlobal().Match(pattern, text)
}

// FindString usa el caché global para buscar coincidencia
func FindString(pattern, text string) string {
	return GetGlobal().FindString(pattern, text)
}

// FindStringSubmatch usa el caché global para buscar con grupos
func FindStringSubmatch(pattern, text string) []string {
	return GetGlobal().FindStringSubmatch(pattern, text)
}
