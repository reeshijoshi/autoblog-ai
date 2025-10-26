package medium

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/yourusername/autoblog-ai/internal/article"
)

// Helper function to create a test publisher with a custom API URL
func newTestPublisher(token, apiURL string) Publisher {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return &mediumPublisher{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
		apiURL: apiURL,
		logger: logger,
	}
}

func TestNewPublisher(t *testing.T) {
	pub := NewPublisher("test-token")

	if pub == nil {
		t.Fatal("NewPublisher() returned nil")
	}
}

func TestGetUser_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/me" {
			t.Errorf("Expected /me, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Invalid authorization header")
		}

		// Send mock response
		response := map[string]interface{}{
			"data": map[string]string{
				"id":       "test-id",
				"username": "testuser",
				"name":     "Test User",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create publisher with mock server URL using helper
	pub := newTestPublisher("test-token", server.URL).(*mediumPublisher)

	// Call getUser
	user, err := pub.getUser(context.Background())
	if err != nil {
		t.Fatalf("getUser() error = %v", err)
	}

	if user.ID != "test-id" {
		t.Errorf("user.ID = %v, want test-id", user.ID)
	}

	if user.Username != "testuser" {
		t.Errorf("user.Username = %v, want testuser", user.Username)
	}

	if user.Name != "Test User" {
		t.Errorf("user.Name = %v, want Test User", user.Name)
	}
}

func TestGetUser_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors": [{"message": "Invalid token"}]}`))
	}))
	defer server.Close()

	pub := newTestPublisher("invalid-token", server.URL).(*mediumPublisher)

	_, err := pub.getUser(context.Background())
	if err == nil {
		t.Error("getUser() should return error for unauthorized request")
	}

	if !contains(err.Error(), "401") {
		t.Errorf("Error should mention status 401, got: %v", err)
	}
}

func TestGetUser_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	pub := newTestPublisher("test-token", server.URL).(*mediumPublisher)

	_, err := pub.getUser(context.Background())
	if err == nil {
		t.Error("getUser() should return error for invalid JSON")
	}
}

func TestPublish_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if callCount == 1 {
			// First call: getUser
			if r.URL.Path != "/me" {
				t.Errorf("Expected /me, got %s", r.URL.Path)
			}

			response := map[string]any{
				"data": map[string]string{
					"id":       "test-user-id",
					"username": "testuser",
					"name":     "Test User",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}
		} else {
			// Second call: publish
			if r.URL.Path != "/users/test-user-id/posts" {
				t.Errorf("Expected /users/test-user-id/posts, got %s", r.URL.Path)
			}

			var reqBody Post
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			// Verify request body
			if reqBody.Title != "Test Article" {
				t.Errorf("Title = %v, want Test Article", reqBody.Title)
			}
			if reqBody.Content == "" {
				t.Error("Missing content in request")
			}
			if reqBody.ContentFormat != "markdown" {
				t.Errorf("ContentFormat = %v, want markdown", reqBody.ContentFormat)
			}
			if reqBody.PublishStatus != "public" {
				t.Errorf("PublishStatus = %v, want public", reqBody.PublishStatus)
			}

			// Send success response
			response := map[string]any{
				"data": map[string]string{
					"id":  "post-123",
					"url": "https://medium.com/@testuser/test-article",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}
		}
	}))
	defer server.Close()

	pub := newTestPublisher("test-token", server.URL)

	art := &article.Article{
		Title:   "Test Article",
		Content: "# Test Content\n\nThis is a test.",
		Tags:    []string{"go", "testing"},
	}

	url, err := pub.Publish(context.Background(), art)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if url != "https://medium.com/@testuser/test-article" {
		t.Errorf("Publish() url = %v, want https://medium.com/@testuser/test-article", url)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls, got %d", callCount)
	}
}

func TestPublish_GetUserError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors": [{"message": "Invalid token"}]}`))
	}))
	defer server.Close()

	pub := newTestPublisher("invalid-token", server.URL)

	art := &article.Article{
		Title:   "Test Article",
		Content: "Content",
		Tags:    []string{"test"},
	}

	_, err := pub.Publish(context.Background(), art)
	if err == nil {
		t.Error("Publish() should return error when getUser fails")
	}

	if !contains(err.Error(), "failed to get user") {
		t.Errorf("Error should mention 'failed to get user', got: %v", err)
	}
}

func TestPublish_PublishError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++

		if callCount == 1 {
			// First call: getUser succeeds
			response := map[string]any{
				"data": map[string]string{
					"id":       "test-user-id",
					"username": "testuser",
					"name":     "Test User",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		} else {
			// Second call: publish fails
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors": [{"message": "Invalid post data"}]}`))
		}
	}))
	defer server.Close()

	pub := newTestPublisher("test-token", server.URL)

	art := &article.Article{
		Title:   "Test Article",
		Content: "Content",
		Tags:    []string{"test"},
	}

	_, err := pub.Publish(context.Background(), art)
	if err == nil {
		t.Error("Publish() should return error when publish request fails")
	}

	if !contains(err.Error(), "failed to publish") {
		t.Errorf("Error should mention 'failed to publish', got: %v", err)
	}
}

func TestPublish_InvalidResponseJSON(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++

		if callCount == 1 {
			// First call: getUser succeeds
			response := map[string]any{
				"data": map[string]string{
					"id":       "test-user-id",
					"username": "testuser",
					"name":     "Test User",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		} else {
			// Second call: publish returns invalid JSON
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`invalid json response`))
		}
	}))
	defer server.Close()

	pub := newTestPublisher("test-token", server.URL)

	art := &article.Article{
		Title:   "Test Article",
		Content: "Content",
		Tags:    []string{"test"},
	}

	_, err := pub.Publish(context.Background(), art)
	if err == nil {
		t.Error("Publish() should return error when response JSON is invalid")
	}
}

func TestMediumPost_Structure(t *testing.T) {
	post := Post{
		Title:         "Test Article",
		ContentFormat: "markdown",
		Content:       "# Test\n\nContent",
		Tags:          []string{"go", "testing"},
		PublishStatus: "public",
	}

	// Marshal to JSON to verify structure
	data, err := json.Marshal(post)
	if err != nil {
		t.Fatalf("Failed to marshal post: %v", err)
	}

	// Unmarshal back
	var decoded Post
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal post: %v", err)
	}

	if decoded.Title != post.Title {
		t.Errorf("Title = %v, want %v", decoded.Title, post.Title)
	}

	if decoded.ContentFormat != post.ContentFormat {
		t.Errorf("ContentFormat = %v, want %v", decoded.ContentFormat, post.ContentFormat)
	}

	if len(decoded.Tags) != len(post.Tags) {
		t.Errorf("Tags length = %v, want %v", len(decoded.Tags), len(post.Tags))
	}
}

func TestPublisher_ArticleConversion(t *testing.T) {
	art := &article.Article{
		Title:       "Test Article",
		Content:     "# Test\n\nContent here",
		Tags:        []string{"go", "test"},
		PublishedAt: time.Now(),
	}

	// Verify article structure is compatible with Medium API
	if art.Title == "" {
		t.Error("Article title is empty")
	}

	if art.Content == "" {
		t.Error("Article content is empty")
	}

	if len(art.Tags) == 0 {
		t.Error("Article has no tags")
	}

	// Medium API expects max 5 tags
	if len(art.Tags) > 5 {
		t.Error("Article has more than 5 tags (Medium limit)")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
