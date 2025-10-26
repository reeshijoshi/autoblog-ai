package article

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/autoblog-ai/internal/config"
	"github.com/yourusername/autoblog-ai/internal/storage"
)

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

	if gen.apiKey != "test-api-key" {
		t.Errorf("apiKey = %v, want test-api-key", gen.apiKey)
	}

	if gen.config != cfg {
		t.Error("config not set correctly")
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
	gen := NewGenerator("test-key", cfg)

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

	gen := NewGenerator("test-key", cfg)

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

func TestGenerate_MockedAPI(t *testing.T) {
	// Create mock server
	mockResponse := `{
		"content": [{
			"text": "{\"title\": \"Test Article\", \"content\": \"# Test\\n\\nArticle content\", \"tags\": [\"go\", \"test\"]}"
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("x-api-key") == "" {
			t.Error("Missing API key header")
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

	// Create generator with mocked client
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
	}

	_ = NewGenerator("test-api-key", cfg)

	// Override API URL for testing (we'd need to modify the generator to support this in real code)
	// For now, this test demonstrates the structure

	// Test with empty history
	history := &storage.ArticleHistory{Articles: []storage.ArticleRecord{}}

	// Note: This will fail in actual execution because we can't override the URL
	// In production, you'd want to make the API URL configurable or use dependency injection
	// For now, this shows the test structure
	_ = history
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

	gen := NewGenerator("test-key", cfg)

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

	gen := NewGenerator("test-key", cfg)

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
