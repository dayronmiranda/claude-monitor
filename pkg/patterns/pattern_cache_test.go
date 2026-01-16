package patterns

import (
	"testing"
)

func TestNew(t *testing.T) {
	pc := New()

	if pc == nil {
		t.Fatal("New should return non-nil PatternCache")
	}

	if pc.Size() != 0 {
		t.Errorf("Size: got %d, want 0", pc.Size())
	}
}

func TestNewWithPatterns(t *testing.T) {
	patterns := []string{`^\d+$`, `[a-z]+`}
	pc := NewWithPatterns(patterns)

	if pc.Size() != 2 {
		t.Errorf("Size: got %d, want 2", pc.Size())
	}
}

func TestPatternCache_Compile(t *testing.T) {
	pc := New()

	re, err := pc.Compile(`^\d+$`)
	if err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	if re == nil {
		t.Fatal("Compile should return non-nil regexp")
	}

	// Second compile should return same instance
	re2, err := pc.Compile(`^\d+$`)
	if err != nil {
		t.Fatalf("Second Compile error: %v", err)
	}

	if re != re2 {
		t.Error("Compile should return cached regexp")
	}
}

func TestPatternCache_Compile_Invalid(t *testing.T) {
	pc := New()

	_, err := pc.Compile(`[invalid`)
	if err == nil {
		t.Error("Compile should return error for invalid pattern")
	}
}

func TestPatternCache_MustCompile(t *testing.T) {
	pc := New()

	re := pc.MustCompile(`^\d+$`)
	if re == nil {
		t.Fatal("MustCompile should return non-nil regexp")
	}
}

func TestPatternCache_MustCompile_Panic(t *testing.T) {
	pc := New()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustCompile should panic for invalid pattern")
		}
	}()

	pc.MustCompile(`[invalid`)
}

func TestPatternCache_Get(t *testing.T) {
	pc := New()

	// Should not exist initially
	_, ok := pc.Get(`^\d+$`)
	if ok {
		t.Error("Get should return false for non-existent pattern")
	}

	// Compile and get
	pc.Compile(`^\d+$`)

	re, ok := pc.Get(`^\d+$`)
	if !ok {
		t.Error("Get should return true after Compile")
	}
	if re == nil {
		t.Error("Get should return non-nil regexp")
	}
}

func TestPatternCache_Match(t *testing.T) {
	pc := New()

	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{`^\d+$`, "123", true},
		{`^\d+$`, "abc", false},
		{`[a-z]+`, "hello", true},
		{`[a-z]+`, "HELLO", false},
		{`(?i)hello`, "HELLO", true},
	}

	for _, tt := range tests {
		got := pc.Match(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("Match(%q, %q): got %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestPatternCache_Match_InvalidPattern(t *testing.T) {
	pc := New()

	got := pc.Match(`[invalid`, "text")
	if got {
		t.Error("Match should return false for invalid pattern")
	}
}

func TestPatternCache_FindString(t *testing.T) {
	pc := New()

	tests := []struct {
		pattern string
		text    string
		want    string
	}{
		{`\d+`, "abc123def", "123"},
		{`\d+`, "abcdef", ""},
		{`[A-Z]+`, "helloWORLD", "WORLD"},
	}

	for _, tt := range tests {
		got := pc.FindString(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("FindString(%q, %q): got %q, want %q", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestPatternCache_FindStringSubmatch(t *testing.T) {
	pc := New()

	result := pc.FindStringSubmatch(`(\d+)-(\d+)`, "100-200")
	if result == nil {
		t.Fatal("FindStringSubmatch should return non-nil for match")
	}

	if len(result) != 3 {
		t.Fatalf("Result length: got %d, want 3", len(result))
	}

	if result[0] != "100-200" {
		t.Errorf("Full match: got %q, want %q", result[0], "100-200")
	}

	if result[1] != "100" {
		t.Errorf("Group 1: got %q, want %q", result[1], "100")
	}

	if result[2] != "200" {
		t.Errorf("Group 2: got %q, want %q", result[2], "200")
	}
}

func TestPatternCache_FindStringSubmatch_NoMatch(t *testing.T) {
	pc := New()

	result := pc.FindStringSubmatch(`\d+`, "abc")
	if result != nil {
		t.Error("FindStringSubmatch should return nil for no match")
	}
}

func TestPatternCache_Size(t *testing.T) {
	pc := New()

	pc.Compile(`pattern1`)
	pc.Compile(`pattern2`)
	pc.Compile(`pattern3`)

	if pc.Size() != 3 {
		t.Errorf("Size: got %d, want 3", pc.Size())
	}
}

func TestPatternCache_Patterns(t *testing.T) {
	pc := New()

	pc.Compile(`a`)
	pc.Compile(`b`)

	patterns := pc.Patterns()
	if len(patterns) != 2 {
		t.Errorf("Patterns length: got %d, want 2", len(patterns))
	}

	pMap := make(map[string]bool)
	for _, p := range patterns {
		pMap[p] = true
	}

	if !pMap["a"] || !pMap["b"] {
		t.Error("Patterns should contain 'a' and 'b'")
	}
}

func TestGetGlobal(t *testing.T) {
	pc1 := GetGlobal()
	pc2 := GetGlobal()

	if pc1 != pc2 {
		t.Error("GetGlobal should return the same instance")
	}

	// Should have pre-compiled Claude patterns
	if pc1.Size() == 0 {
		t.Error("Global cache should have pre-compiled patterns")
	}
}

func TestGlobalHelpers(t *testing.T) {
	// Test global Match
	if !Match(`\d+`, "123") {
		t.Error("Global Match failed for valid pattern")
	}

	// Test global FindString
	if FindString(`\d+`, "abc123") != "123" {
		t.Error("Global FindString failed")
	}

	// Test global FindStringSubmatch
	result := FindStringSubmatch(`(\d+)`, "abc123")
	if result == nil || result[1] != "123" {
		t.Error("Global FindStringSubmatch failed")
	}
}

func TestClaudePatterns(t *testing.T) {
	pc := GetGlobal()

	// Test some Claude-specific patterns
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{"permission allow", `(?i)Allow\s+\w+.*to`, "Allow Edit to write file.txt", true},
		{"y/n prompt", `\[y/n\]`, "Continue? [y/n]", true},
		{"spinner", `[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷]`, "⠋ Loading...", true},
		{"prompt", `^>\s*$`, "> ", true},
		{"vim insert", `-- INSERT --`, "-- INSERT --", true},
		{"error", `(?i)^Error:`, "Error: file not found", true},
		{"tokens", `(?i)tokens?:`, "Tokens: 1234", true},
		{"slash command", `^/\w+`, "/help", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pc.Match(tt.pattern, tt.text)
			if got != tt.want {
				t.Errorf("Match(%q, %q): got %v, want %v", tt.pattern, tt.text, got, tt.want)
			}
		})
	}
}

func TestPatternCache_Concurrent(t *testing.T) {
	pc := New()
	done := make(chan bool)

	// Concurrent compilers
	go func() {
		for i := 0; i < 100; i++ {
			pc.Compile(`\d+`)
		}
		done <- true
	}()

	// Concurrent matchers
	go func() {
		for i := 0; i < 100; i++ {
			pc.Match(`\d+`, "123")
		}
		done <- true
	}()

	<-done
	<-done

	// Should not panic
}

func BenchmarkPatternCache_Match(b *testing.B) {
	pc := New()
	pc.Compile(`\d+`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pc.Match(`\d+`, "abc123def")
	}
}

func BenchmarkRawRegexp_Match(b *testing.B) {
	pc := New()
	re := pc.MustCompile(`\d+`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString("abc123def")
	}
}
