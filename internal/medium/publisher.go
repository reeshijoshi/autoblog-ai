// Package medium provides functionality for publishing articles to Medium.
package medium

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yourusername/autoblog-ai/internal/article"
)

// Publisher is an interface for publishing articles to Medium.
type Publisher interface {
	Publish(ctx context.Context, article *article.Article) (string, error)
}

// mediumPublisher is the concrete implementation of Publisher.
type mediumPublisher struct {
	token  string
	client *http.Client
	apiURL string
	logger *slog.Logger
}

// User represents a Medium user account.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// Post represents a post to be published on Medium.
type Post struct {
	Title         string   `json:"title"`
	ContentFormat string   `json:"contentFormat"`
	Content       string   `json:"content"`
	Tags          []string `json:"tags,omitempty"`
	PublishStatus string   `json:"publishStatus"`
}

// NewPublisher creates a new Medium publisher with the given API token.
func NewPublisher(token string) Publisher {
	logger := slog.Default().With("component", "medium.publisher")
	return &mediumPublisher{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
		apiURL: "https://api.medium.com/v1",
		logger: logger,
	}
}

// NewPublisherWithLogger creates a new Medium publisher with a custom logger.
func NewPublisherWithLogger(token string, logger *slog.Logger) Publisher {
	return &mediumPublisher{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
		apiURL: "https://api.medium.com/v1",
		logger: logger.With("component", "medium.publisher"),
	}
}

// Publish publishes an article to Medium and returns the URL of the published post.
func (p *mediumPublisher) Publish(ctx context.Context, article *article.Article) (string, error) {
	logger := p.logger.With(
		"article_title", article.Title,
		"tags_count", len(article.Tags),
		"content_length", len(article.Content),
	)
	logger.InfoContext(ctx, "Starting article publication to Medium")

	// First, get the user ID
	logger.DebugContext(ctx, "Fetching Medium user information")
	user, err := p.getUser(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get Medium user", "error", err)
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	logger.InfoContext(ctx, "Successfully retrieved user information",
		"user_id", user.ID,
		"username", user.Username)

	// Create the post
	post := Post{
		Title:         article.Title,
		ContentFormat: "markdown",
		Content:       article.Content,
		Tags:          article.Tags,
		PublishStatus: "public", // Can be "public", "draft", or "unlisted"
	}

	url := fmt.Sprintf("%s/users/%s/posts", p.apiURL, user.ID)
	logger.DebugContext(ctx, "Prepared post data", "api_url", url)
	jsonData, err := json.Marshal(post)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to marshal post data", "error", err)
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create HTTP request", "error", err)
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	logger.InfoContext(ctx, "Sending publish request to Medium API")
	start := time.Now()
	resp, err := p.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		logger.ErrorContext(ctx, "HTTP request failed",
			"error", err,
			"duration_ms", duration.Milliseconds())
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	logger.DebugContext(ctx, "Received response from Medium API",
		"status_code", resp.StatusCode,
		"duration_ms", duration.Milliseconds())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read response body", "error", err)
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		logger.ErrorContext(ctx, "Publication failed",
			"status_code", resp.StatusCode,
			"response_body", string(body))
		return "", fmt.Errorf("failed to publish (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logger.ErrorContext(ctx, "Failed to unmarshal response", "error", err)
		return "", err
	}

	logger.InfoContext(ctx, "Successfully published article to Medium",
		"published_url", result.Data.URL)

	return result.Data.URL, nil
}

func (p *mediumPublisher) getUser(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/me", p.apiURL), nil)
	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to create user info request", "error", err)
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	req.Header.Set("Accept", "application/json")

	p.logger.DebugContext(ctx, "Fetching user info from Medium API")
	start := time.Now()
	resp, err := p.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to fetch user info",
			"error", err,
			"duration_ms", duration.Milliseconds())
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	p.logger.DebugContext(ctx, "Received user info response",
		"status_code", resp.StatusCode,
		"duration_ms", duration.Milliseconds())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to read user info response", "error", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		p.logger.ErrorContext(ctx, "Failed to get user info",
			"status_code", resp.StatusCode,
			"response_body", string(body))
		return nil, fmt.Errorf("failed to get user (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data User `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		p.logger.ErrorContext(ctx, "Failed to unmarshal user info", "error", err)
		return nil, err
	}

	p.logger.DebugContext(ctx, "Successfully retrieved user info",
		"user_id", result.Data.ID,
		"username", result.Data.Username)

	return &result.Data, nil
}

var _ Publisher = &mediumPublisher{}
