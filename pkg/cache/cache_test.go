package cache

import (
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	if c == nil {
		t.Fatal("New should return non-nil cache")
	}

	if c.Size() != 0 {
		t.Errorf("Size: got %d, want 0", c.Size())
	}
}

func TestCache_SetAndGet(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	c.Set("key1", "value1")

	value, ok := c.Get("key1")
	if !ok {
		t.Error("Get should return true for existing key")
	}

	if value != "value1" {
		t.Errorf("Value: got %s, want %s", value, "value1")
	}
}

func TestCache_GetNonExistent(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent key")
	}
}

func TestCache_Delete(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("Get should return false after Delete")
	}
}

func TestCache_Clear(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Clear()

	if c.Size() != 0 {
		t.Errorf("Size after Clear: got %d, want 0", c.Size())
	}
}

func TestCache_Size(t *testing.T) {
	c := New[int](time.Minute, 100)
	defer c.Stop()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	if c.Size() != 3 {
		t.Errorf("Size: got %d, want 3", c.Size())
	}
}

func TestCache_Keys(t *testing.T) {
	c := New[int](time.Minute, 100)
	defer c.Stop()

	c.Set("a", 1)
	c.Set("b", 2)

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys length: got %d, want 2", len(keys))
	}

	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	if !keyMap["a"] || !keyMap["b"] {
		t.Error("Keys should contain 'a' and 'b'")
	}
}

func TestCache_Expiration(t *testing.T) {
	c := New[string](50*time.Millisecond, 100)
	defer c.Stop()

	c.Set("key1", "value1")

	// Should exist immediately
	_, ok := c.Get("key1")
	if !ok {
		t.Error("Get should return true immediately after Set")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should be expired
	_, ok = c.Get("key1")
	if ok {
		t.Error("Get should return false after expiration")
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	// Set with short TTL
	c.SetWithTTL("key1", "value1", 50*time.Millisecond)

	_, ok := c.Get("key1")
	if !ok {
		t.Error("Get should return true immediately")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get("key1")
	if ok {
		t.Error("Get should return false after custom TTL expiration")
	}
}

func TestCache_MaxSize_Eviction(t *testing.T) {
	c := New[int](time.Minute, 3)
	defer c.Stop()

	c.Set("a", 1)
	time.Sleep(1 * time.Millisecond) // Ensure different access times
	c.Set("b", 2)
	time.Sleep(1 * time.Millisecond)
	c.Set("c", 3)

	// Access 'a' to make it recently used
	c.Get("a")
	time.Sleep(1 * time.Millisecond)

	// This should evict the oldest entry
	c.Set("d", 4)

	if c.Size() != 3 {
		t.Errorf("Size: got %d, want 3", c.Size())
	}

	// 'd' should exist
	_, ok := c.Get("d")
	if !ok {
		t.Error("'d' should exist after eviction")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	callCount := 0
	generator := func() (string, error) {
		callCount++
		return "generated", nil
	}

	// First call should invoke generator
	value, err := c.GetOrSet("key1", generator)
	if err != nil {
		t.Fatalf("GetOrSet error: %v", err)
	}
	if value != "generated" {
		t.Errorf("Value: got %s, want %s", value, "generated")
	}
	if callCount != 1 {
		t.Errorf("Call count: got %d, want 1", callCount)
	}

	// Second call should use cached value
	value, err = c.GetOrSet("key1", generator)
	if err != nil {
		t.Fatalf("GetOrSet error: %v", err)
	}
	if value != "generated" {
		t.Errorf("Value: got %s, want %s", value, "generated")
	}
	if callCount != 1 {
		t.Errorf("Call count after second call: got %d, want 1", callCount)
	}
}

func TestCache_GetOrSet_Error(t *testing.T) {
	c := New[string](time.Minute, 100)
	defer c.Stop()

	expectedErr := errors.New("generator error")
	generator := func() (string, error) {
		return "", expectedErr
	}

	_, err := c.GetOrSet("key1", generator)
	if err != expectedErr {
		t.Errorf("Error: got %v, want %v", err, expectedErr)
	}

	// Should not be cached
	if c.Size() != 0 {
		t.Error("Failed generator result should not be cached")
	}
}

func TestCache_Stats(t *testing.T) {
	c := New[int](time.Minute, 50)
	defer c.Stop()

	c.Set("a", 1)
	c.Set("b", 2)

	stats := c.Stats()

	if stats.Size != 2 {
		t.Errorf("Stats.Size: got %d, want 2", stats.Size)
	}

	if stats.MaxSize != 50 {
		t.Errorf("Stats.MaxSize: got %d, want 50", stats.MaxSize)
	}

	if stats.TTLMillis != 60000 {
		t.Errorf("Stats.TTLMillis: got %d, want 60000", stats.TTLMillis)
	}
}

func TestCache_Concurrent(t *testing.T) {
	c := New[int](time.Minute, 1000)
	defer c.Stop()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			c.Set("key", i)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			c.Get("key")
		}
		done <- true
	}()

	<-done
	<-done

	// Should not panic
}

func TestCache_TypedValues(t *testing.T) {
	t.Run("struct type", func(t *testing.T) {
		type User struct {
			ID   int
			Name string
		}

		c := New[User](time.Minute, 100)
		defer c.Stop()

		c.Set("user1", User{ID: 1, Name: "Alice"})

		user, ok := c.Get("user1")
		if !ok {
			t.Fatal("Get should return true")
		}

		if user.ID != 1 || user.Name != "Alice" {
			t.Errorf("User: got %+v, want {ID:1 Name:Alice}", user)
		}
	})

	t.Run("slice type", func(t *testing.T) {
		c := New[[]int](time.Minute, 100)
		defer c.Stop()

		c.Set("nums", []int{1, 2, 3})

		nums, ok := c.Get("nums")
		if !ok {
			t.Fatal("Get should return true")
		}

		if len(nums) != 3 {
			t.Errorf("Slice length: got %d, want 3", len(nums))
		}
	})
}
