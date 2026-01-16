package cache

import (
	"sync"
	"time"
)

// Cache es una caché genérica con TTL
type Cache[T any] struct {
	data    map[string]*cacheEntry[T]
	mu      sync.RWMutex
	ttl     time.Duration
	maxSize int
	done    chan struct{}
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
	accessedAt time.Time
}

// New crea una nueva caché
func New[T any](ttl time.Duration, maxSize int) *Cache[T] {
	c := &Cache[T]{
		data:    make(map[string]*cacheEntry[T]),
		ttl:     ttl,
		maxSize: maxSize,
		done:    make(chan struct{}),
	}

	// Iniciar goroutine de limpieza
	go c.cleanup()

	return c
}

// Get obtiene un valor de la caché
func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		var zero T
		return zero, false
	}

	// Verificar expiración
	if time.Now().After(entry.expiresAt) {
		var zero T
		return zero, false
	}

	// Actualizar tiempo de acceso (para LRU)
	entry.accessedAt = time.Now()

	return entry.value, true
}

// Set almacena un valor en la caché
func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evictar si está llena
	if len(c.data) >= c.maxSize {
		c.evictOldest()
	}

	c.data[key] = &cacheEntry[T]{
		value:      value,
		expiresAt:  time.Now().Add(c.ttl),
		accessedAt: time.Now(),
	}
}

// SetWithTTL almacena un valor con TTL personalizado
func (c *Cache[T]) SetWithTTL(key string, value T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.data) >= c.maxSize {
		c.evictOldest()
	}

	c.data[key] = &cacheEntry[T]{
		value:      value,
		expiresAt:  time.Now().Add(ttl),
		accessedAt: time.Now(),
	}
}

// Delete elimina un valor de la caché
func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear limpia toda la caché
func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]*cacheEntry[T])
}

// Size retorna el número de elementos en la caché
func (c *Cache[T]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// Keys retorna las claves de la caché
func (c *Cache[T]) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	return keys
}

// Stop detiene la goroutine de limpieza
func (c *Cache[T]) Stop() {
	close(c.done)
}

// evictOldest elimina el elemento más antiguo (LRU)
func (c *Cache[T]) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.data {
		if oldestKey == "" || entry.accessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.accessedAt
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// cleanup limpia entradas expiradas periódicamente
func (c *Cache[T]) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.done:
			return
		}
	}
}

// removeExpired elimina entradas expiradas
func (c *Cache[T]) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.data {
		if now.After(entry.expiresAt) {
			delete(c.data, key)
		}
	}
}

// GetOrSet obtiene un valor o lo calcula si no existe
func (c *Cache[T]) GetOrSet(key string, fn func() (T, error)) (T, error) {
	// Primero intentar obtener
	if value, ok := c.Get(key); ok {
		return value, nil
	}

	// Si no existe, calcular
	value, err := fn()
	if err != nil {
		var zero T
		return zero, err
	}

	// Almacenar
	c.Set(key, value)
	return value, nil
}

// Stats estadísticas de la caché
type Stats struct {
	Size      int `json:"size"`
	MaxSize   int `json:"max_size"`
	TTLMillis int `json:"ttl_millis"`
}

// Stats retorna estadísticas de la caché
func (c *Cache[T]) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Stats{
		Size:      len(c.data),
		MaxSize:   c.maxSize,
		TTLMillis: int(c.ttl.Milliseconds()),
	}
}
