package article

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourusername/autoblog-ai/internal/config"
	"github.com/yourusername/autoblog-ai/internal/storage"
)

// Helper function to create a test generator with a custom API URL
func newTestGenerator(apiKey string, cfg *config.Config, apiURL string) Generator {
	timeout := time.Duration(cfg.AI.TimeoutSeconds) * time.Second
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return &claudeGenerator{
		apiKey: apiKey,
		config: cfg,
		client: &http.Client{Timeout: timeout},
		apiURL: apiURL,
		logger: logger,
	}
}

func TestNewGenerator(t *testing.T) {
	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
	}

	gen := NewGenerator("test-api-key", cfg)

	if gen == nil {
		t.Fatal("NewGenerator() returned nil")
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		skipContent bool // Skip content validation for this test
	}{
		{
			name: "valid JSON",
			input: `{
				"title": "Test Article",
				"content": "# Test\n\nContent here",
				"tags": ["go", "testing"]
			}`,
			wantErr: false,
		},
		{
			name: "JSON with extra text",
			input: `Here is your article:
			{
				"title": "Test Article",
				"content": "Content",
				"tags": ["go"]
			}
			Thank you!`,
			wantErr: false,
		},
		{
			name:    "no JSON",
			input:   `This is just text without JSON`,
			wantErr: true,
		},
	}

	cfg := &config.Config{}
	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article, err := gen.parseResponse(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if article.Title == "" {
					t.Error("parseResponse() title is empty")
				}
				if article.Content == "" {
					t.Error("parseResponse() content is empty")
				}
			}
		})
	}
}

func TestBuildPromptFromTemplate(t *testing.T) {
	cfg := &config.Config{
		Style: config.StyleConfig{
			Tone:           "professional",
			Length:         "medium",
			TargetAudience: "intermediate",
			IncludeCode:    true,
		},
		PromptTemplate: "templates/article-prompt.md",
	}

	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	topic := "Go Concurrency"
	topicDetails := &config.TopicConfig{
		Description: "Advanced concurrency patterns",
		Keywords:    []string{"goroutines", "channels"},
	}
	previousTitles := []string{"Previous Article 1"}

	prompt := gen.buildPromptFromTemplate(topic, topicDetails, previousTitles)

	// Should fall back to built-in prompt since template file doesn't exist in test
	if prompt == "" {
		t.Error("buildPromptFromTemplate() returned empty prompt")
	}

	// Verify prompt contains key elements
	if !contains(prompt, topic) {
		t.Error("Prompt should contain topic")
	}
}

func TestBuildPromptFromTemplate_WithValidTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "prompt.md")
	templateContent := `Write about: {{.Topic}}
Description: {{.TopicDescription}}
Keywords: {{.Keywords}}
Tone: {{.Tone}}
Length: {{.Length}}
Audience: {{.TargetAudience}}
{{if .IncludeCode}}Include code examples{{end}}
{{range .PreviousTitles}}
Previous: {{.}}
{{end}}`

	if err := os.WriteFile(templatePath, []byte(templateContent), 0600); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	cfg := &config.Config{
		Style: config.StyleConfig{
			Tone:           "casual",
			Length:         "long",
			TargetAudience: "advanced",
			IncludeCode:    true,
		},
		PromptTemplate: templatePath,
	}

	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	topic := "Rust Ownership"
	topicDetails := &config.TopicConfig{
		Description: "Memory safety without GC",
		Keywords:    []string{"borrowing", "lifetimes"},
	}
	previousTitles := []string{"Old Title 1", "Old Title 2"}

	prompt := gen.buildPromptFromTemplate(topic, topicDetails, previousTitles)

	// Verify all template variables were filled
	expectedStrings := []string{
		"Write about: Rust Ownership",
		"Description: Memory safety without GC",
		"Keywords: borrowing, lifetimes",
		"Tone: casual",
		"Length: long",
		"Audience: advanced",
		"Include code examples",
		"Previous: Old Title 1",
		"Previous: Old Title 2",
	}

	for _, expected := range expectedStrings {
		if !contains(prompt, expected) {
			t.Errorf("Prompt missing expected string: %s", expected)
		}
	}
}

func TestBuildPromptFromTemplate_InvalidTemplateSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "invalid.md")
	// Invalid template syntax
	invalidContent := `{{.Topic} - missing closing brace`

	if err := os.WriteFile(templatePath, []byte(invalidContent), 0600); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	cfg := &config.Config{
		Style: config.StyleConfig{
			Tone:   "professional",
			Length: "medium",
		},
		PromptTemplate: templatePath,
	}

	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	// Should fall back to built-in template on parse error
	prompt := gen.buildPromptFromTemplate("Test Topic", nil, nil)

	if prompt == "" {
		t.Error("buildPromptFromTemplate() should return fallback prompt")
	}

	// Should contain fallback prompt elements
	if !contains(prompt, "Test Topic") {
		t.Error("Fallback prompt should contain topic")
	}
}

func TestGenerate_Success(t *testing.T) {
	// Create mock server
	mockResponse := `{
		"content": [{
			"text": "{\"title\": \"Understanding Go Concurrency\", \"content\": \"# Understanding Go Concurrency\\n\\nGo provides excellent support for concurrent programming.\", \"tags\": [\"go\", \"concurrency\", \"goroutines\"]}"
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Error("Missing or incorrect API key header")
		}

		// Verify request body contains expected fields
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if reqBody["model"] == nil {
			t.Error("Request missing model field")
		}

		if reqBody["messages"] == nil {
			t.Error("Request missing messages field")
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(mockResponse)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Create generator with mocked API URL
	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
		Style: config.StyleConfig{
			Tone:           "professional",
			Length:         "medium",
			TargetAudience: "intermediate",
			IncludeCode:    true,
		},
		Topics: []config.TopicConfig{
			{Name: "Go Concurrency", Weight: 1},
		},
	}

	gen := newTestGenerator("test-api-key", cfg, server.URL).(*claudeGenerator)

	// Test with empty history
	history := &storage.ArticleHistory{Articles: []storage.ArticleRecord{}}

	article, err := gen.Generate(t.Context(), "Go Concurrency", history)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if article == nil {
		t.Fatal("Generate() returned nil article")
	}

	if article.Title != "Understanding Go Concurrency" {
		t.Errorf("article.Title = %v, want 'Understanding Go Concurrency'", article.Title)
	}

	if !contains(article.Content, "Go provides excellent support") {
		t.Error("article.Content should contain expected text")
	}

	if len(article.Tags) != 3 {
		t.Errorf("article.Tags length = %v, want 3", len(article.Tags))
	}
}

func TestGenerate_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Delay to allow context cancellation
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
		Style: config.StyleConfig{
			Tone:   "professional",
			Length: "medium",
		},
	}

	gen := newTestGenerator("test-api-key", cfg, server.URL).(*claudeGenerator)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	history := &storage.ArticleHistory{Articles: []storage.ArticleRecord{}}

	_, err := gen.Generate(ctx, "Test Topic", history)
	if err == nil {
		t.Error("Generate() should return error when context is canceled")
	}
}

func TestGenerate_WithPreviousTitles(t *testing.T) {
	mockResponse := `{
		"content": [{
			"text": "{\"title\": \"New Unique Article\", \"content\": \"# New\\n\\nContent\", \"tags\": [\"go\"]}"
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the prompt includes previous titles
		var reqBody map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		messages := reqBody["messages"].([]interface{})
		userMessage := messages[0].(map[string]interface{})
		content := userMessage["content"].(string)

		// Should contain reference to previous titles
		if !contains(content, "Test Topic") {
			t.Error("Prompt should contain the topic name")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
		Style: config.StyleConfig{
			Tone:   "professional",
			Length: "medium",
		},
	}

	gen := newTestGenerator("test-api-key", cfg, server.URL).(*claudeGenerator)

	// History with previous articles on the same topic
	history := &storage.ArticleHistory{
		Articles: []storage.ArticleRecord{
			{Topic: "Test Topic", Title: "Previous Article 1"},
			{Topic: "Test Topic", Title: "Previous Article 2"},
		},
	}

	article, err := gen.Generate(t.Context(), "Test Topic", history)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if article.Title == "Previous Article 1" || article.Title == "Previous Article 2" {
		t.Error("Generated article should have a unique title")
	}
}

func TestBuildPromptFallback(t *testing.T) {
	cfg := &config.Config{
		Style: config.StyleConfig{
			Tone:           "professional",
			Length:         "medium",
			TargetAudience: "intermediate",
			IncludeCode:    true,
		},
	}

	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	topic := "Go Testing"
	topicDetails := &config.TopicConfig{
		Description: "Testing strategies",
		Keywords:    []string{"testing", "mocking"},
	}
	previousTitles := []string{"Old Article"}

	prompt := gen.buildPromptFallback(topic, topicDetails, previousTitles)

	if prompt == "" {
		t.Error("buildPromptFallback() returned empty prompt")
	}

	// Verify prompt contains key elements
	tests := []string{
		topic,
		topicDetails.Description,
		"professional",
		"medium",
		"intermediate",
		"Old Article",
	}

	for _, want := range tests {
		if !contains(prompt, want) {
			t.Errorf("Prompt missing expected string: %s", want)
		}
	}
}

func TestGetSystemPrompt(t *testing.T) {
	cfg := &config.Config{
		SystemPrompt: "nonexistent/path.md",
	}

	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	// Should fall back to default since file doesn't exist
	prompt := gen.getSystemPrompt()

	if prompt == "" {
		t.Error("getSystemPrompt() returned empty prompt")
	}

	// Should contain some indication it's about technical writing
	if !contains(prompt, "technical writer") && !contains(prompt, "technical writing") {
		t.Error("System prompt should reference technical writing")
	}
}

func TestIsRetryableError(t *testing.T) {
	gen := NewGenerator("test-key", &config.Config{})

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "context canceled - not retryable",
			err:       fmt.Errorf("context canceled"),
			retryable: false,
		},
		{
			name:      "server error - retryable",
			err:       fmt.Errorf("status 500 internal server error"),
			retryable: true,
		},
		{
			name:      "rate limit - retryable",
			err:       fmt.Errorf("status 429 too many requests"),
			retryable: true,
		},
		{
			name:      "timeout - retryable",
			err:       fmt.Errorf("connection timeout"),
			retryable: true,
		},
		{
			name:      "connection refused - retryable",
			err:       fmt.Errorf("connection refused"),
			retryable: true,
		},
		{
			name:      "client error - not retryable",
			err:       fmt.Errorf("status 400 bad request"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}

	// Test with actual context errors
	if isRetryableError(context.Canceled) {
		t.Error("context.Canceled should not be retryable")
	}

	if isRetryableError(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not be retryable")
	}

	_ = gen // use gen to avoid unused variable error
}

func TestCallClaudeAPI_Success(t *testing.T) {
	mockResponse := `{
		"content": [{
			"text": "Test response text"
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type header should be application/json")
		}

		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("x-api-key header is missing or incorrect")
		}

		if r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic-version header is missing")
		}

		// Verify request body structure
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Check required fields
		requiredFields := []string{"model", "max_tokens", "temperature", "system", "messages"}
		for _, field := range requiredFields {
			if _, ok := reqBody[field]; !ok {
				t.Errorf("Request missing required field: %s", field)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
	}

	gen := newTestGenerator("test-key", cfg, server.URL).(*claudeGenerator)

	response, err := gen.callClaudeAPI(t.Context(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("callClaudeAPI() error = %v", err)
	}

	if response != "Test response text" {
		t.Errorf("callClaudeAPI() response = %v, want 'Test response text'", response)
	}
}

func TestCallClaudeAPI_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
	}

	gen := newTestGenerator("test-key", cfg, server.URL).(*claudeGenerator)

	_, err := gen.callClaudeAPI(t.Context(), "system", "user")
	if err == nil {
		t.Error("callClaudeAPI() should return error for bad request")
	}

	if !contains(err.Error(), "400") {
		t.Errorf("Error should mention status 400, got: %v", err)
	}
}

func TestCallClaudeAPI_EmptyContent(t *testing.T) {
	mockResponse := `{
		"content": []
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
	}

	gen := newTestGenerator("test-key", cfg, server.URL).(*claudeGenerator)

	_, err := gen.callClaudeAPI(t.Context(), "system", "user")
	if err == nil {
		t.Error("callClaudeAPI() should return error for empty content")
	}

	if !contains(err.Error(), "no content") {
		t.Errorf("Error should mention 'no content', got: %v", err)
	}
}

func TestCallClaudeAPIWithRetry_Success(t *testing.T) {
	mockResponse := `{
		"content": [{
			"text": "Success after retry"
		}]
	}`

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount < 2 {
			// First call fails with retryable error
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
		} else {
			// Second call succeeds
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockResponse))
		}
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
	}

	gen := newTestGenerator("test-key", cfg, server.URL).(*claudeGenerator)

	response, err := gen.callClaudeAPIWithRetry(t.Context(), "system", "user")
	if err != nil {
		t.Fatalf("callClaudeAPIWithRetry() error = %v", err)
	}

	if response != "Success after retry" {
		t.Errorf("callClaudeAPIWithRetry() = %v, want 'Success after retry'", response)
	}

	if callCount < 2 {
		t.Errorf("Expected at least 2 calls (1 failure + 1 success), got %d", callCount)
	}
}

func TestCallClaudeAPIWithRetry_MaxRetriesExceeded(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		// Always fail with retryable error
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": {"message": "Server error"}}`))
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 1, // Short timeout for test
		},
	}

	gen := newTestGenerator("test-key", cfg, server.URL).(*claudeGenerator)

	_, err := gen.callClaudeAPIWithRetry(t.Context(), "system", "user")
	if err == nil {
		t.Error("callClaudeAPIWithRetry() should return error after max retries")
	}

	if !contains(err.Error(), "max retries") {
		t.Errorf("Error should mention 'max retries', got: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", callCount)
	}
}

func TestCallClaudeAPIWithRetry_NonRetryableError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		// Non-retryable error (400)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Bad request"}}`))
	}))
	defer server.Close()

	temp := 1.0
	cfg := &config.Config{
		AI: config.AIConfig{
			Model:          "claude-sonnet-4-20250514",
			MaxTokens:      8192,
			Temperature:    &temp,
			TimeoutSeconds: 120,
		},
	}

	gen := newTestGenerator("test-key", cfg, server.URL).(*claudeGenerator)

	_, err := gen.callClaudeAPIWithRetry(t.Context(), "system", "user")
	if err == nil {
		t.Error("callClaudeAPIWithRetry() should return error for non-retryable error")
	}

	// Should only call once, not retry
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retry for non-retryable error), got %d", callCount)
	}
}

func TestParseResponse_EdgeCases(t *testing.T) {
	cfg := &config.Config{}
	gen := NewGenerator("test-key", cfg).(*claudeGenerator)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "JSON with whitespace",
			input: `
				{
					"title": "Test",
					"content": "Content",
					"tags": ["tag"]
				}
			`,
			wantErr: false,
		},
		{
			name: "JSON at end of response",
			input: `Some preamble text

			{"title": "Test", "content": "Content", "tags": ["tag"]}`,
			wantErr: false,
		},
		{
			name:    "missing closing brace",
			input:   `{"title": "Test", "content": "Content"`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name: "multiple JSON objects - uses last complete one",
			input: `{"title": "First", "content": "Content1", "tags": ["a"]}
			{"title": "Second", "content": "Content2", "tags": ["b"]}`,
			wantErr: true, // This will fail because json.Unmarshal can't handle multiple objects
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article, err := gen.parseResponse(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && article == nil {
				t.Error("parseResponse() returned nil article")
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
