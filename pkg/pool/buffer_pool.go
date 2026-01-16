package pool

import (
	"sync"
)

// DefaultBufferSize tamaño por defecto de buffer
const DefaultBufferSize = 4096

// BufferPool pool de buffers para PTY I/O
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool crea un nuevo buffer pool
func NewBufferPool(size int) *BufferPool {
	if size <= 0 {
		size = DefaultBufferSize
	}

	return &BufferPool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get obtiene un buffer del pool
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put devuelve un buffer al pool
func (p *BufferPool) Put(buf []byte) {
	// Solo devolver buffers del tamaño correcto
	if cap(buf) >= p.size {
		// Limpiar el buffer antes de devolverlo
		for i := range buf {
			buf[i] = 0
		}
		p.pool.Put(buf[:p.size])
	}
}

// Size retorna el tamaño de los buffers
func (p *BufferPool) Size() int {
	return p.size
}

// Global buffer pool instance
var (
	globalPool     *BufferPool
	globalPoolOnce sync.Once
)

// GetGlobal retorna el pool global
func GetGlobal() *BufferPool {
	globalPoolOnce.Do(func() {
		globalPool = NewBufferPool(DefaultBufferSize)
	})
	return globalPool
}

// GetBuffer obtiene un buffer del pool global
func GetBuffer() []byte {
	return GetGlobal().Get()
}

// PutBuffer devuelve un buffer al pool global
func PutBuffer(buf []byte) {
	GetGlobal().Put(buf)
}
