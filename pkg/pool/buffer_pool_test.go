package pool

import (
	"testing"
)

func TestNewBufferPool(t *testing.T) {
	pool := NewBufferPool(1024)

	if pool == nil {
		t.Fatal("NewBufferPool should return non-nil pool")
	}

	if pool.Size() != 1024 {
		t.Errorf("Size: got %d, want 1024", pool.Size())
	}
}

func TestNewBufferPool_DefaultSize(t *testing.T) {
	pool := NewBufferPool(0)

	if pool.Size() != DefaultBufferSize {
		t.Errorf("Size: got %d, want %d", pool.Size(), DefaultBufferSize)
	}

	pool2 := NewBufferPool(-1)
	if pool2.Size() != DefaultBufferSize {
		t.Errorf("Size for negative: got %d, want %d", pool2.Size(), DefaultBufferSize)
	}
}

func TestBufferPool_GetAndPut(t *testing.T) {
	pool := NewBufferPool(1024)

	buf := pool.Get()
	if buf == nil {
		t.Fatal("Get should return non-nil buffer")
	}

	if len(buf) != 1024 {
		t.Errorf("Buffer length: got %d, want 1024", len(buf))
	}

	// Write some data
	copy(buf, []byte("hello"))

	// Put it back
	pool.Put(buf)

	// Get a new buffer - it might be the same one
	buf2 := pool.Get()
	if buf2 == nil {
		t.Fatal("Get after Put should return non-nil buffer")
	}

	if len(buf2) != 1024 {
		t.Errorf("Buffer2 length: got %d, want 1024", len(buf2))
	}

	// Buffer should be cleared
	allZero := true
	for _, b := range buf2 {
		if b != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("Buffer should be cleared after Put")
	}
}

func TestBufferPool_WrongSizeBuffer(t *testing.T) {
	pool := NewBufferPool(1024)

	// Create a smaller buffer
	smallBuf := make([]byte, 512)

	// Put it back - should be ignored
	pool.Put(smallBuf)

	// Get should still work
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("Buffer length: got %d, want 1024", len(buf))
	}
}

func TestGetGlobal(t *testing.T) {
	pool1 := GetGlobal()
	pool2 := GetGlobal()

	if pool1 != pool2 {
		t.Error("GetGlobal should return the same instance")
	}

	if pool1.Size() != DefaultBufferSize {
		t.Errorf("Global pool size: got %d, want %d", pool1.Size(), DefaultBufferSize)
	}
}

func TestGlobalHelpers(t *testing.T) {
	buf := GetBuffer()
	if buf == nil {
		t.Fatal("GetBuffer should return non-nil buffer")
	}

	if len(buf) != DefaultBufferSize {
		t.Errorf("Buffer length: got %d, want %d", len(buf), DefaultBufferSize)
	}

	// Should not panic
	PutBuffer(buf)
}

func TestBufferPool_Concurrent(t *testing.T) {
	pool := NewBufferPool(1024)

	done := make(chan bool)
	workers := 10
	iterations := 100

	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				buf := pool.Get()
				// Write some data
				copy(buf, []byte("test data"))
				pool.Put(buf)
			}
			done <- true
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
	}

	// Should not panic or have race conditions
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkRawAllocation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 4096)
		_ = buf
	}
}
