// Package medium provides functionality for publishing articles to Medium.
package medium

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yourusername/autoblog-ai/internal/article"
)

// Publisher handles publishing articles to Medium.
type Publisher struct {
	token  string
	client *http.Client
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
func NewPublisher(token string) *Publisher {
	return &Publisher{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Publish publishes an article to Medium and returns the URL of the published post.
func (p *Publisher) Publish(article *article.Article) (string, error) {
	// First, get the user ID
	user, err := p.getUser()
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	// Create the post
	post := Post{
		Title:         article.Title,
		ContentFormat: "markdown",
		Content:       article.Content,
		Tags:          article.Tags,
		PublishStatus: "public", // Can be "public", "draft", or "unlisted"
	}

	url := fmt.Sprintf("https://api.medium.com/v1/users/%s/posts", user.ID)
	jsonData, err := json.Marshal(post)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to publish (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.Data.URL, nil
}

func (p *Publisher) getUser() (*User, error) {
	req, err := http.NewRequest("GET", "https://api.medium.com/v1/me", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data User `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}
