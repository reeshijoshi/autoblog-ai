// Package article provides functionality for generating articles using AI.
package article

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/yourusername/autoblog-ai/internal/config"
	"github.com/yourusername/autoblog-ai/internal/storage"
)

// Article represents a generated article with metadata.
type Article struct {
	Title       string
	Content     string
	Tags        []string
	PublishedAt time.Time
}

// Generator is an interface for generating articles using AI.
type Generator interface {
	Generate(ctx context.Context, topic string, history *storage.ArticleHistory) (*Article, error)
}

// claudeGenerator is the concrete implementation of Generator using Claude API.
type claudeGenerator struct {
	apiKey string
	config *config.Config
	client *http.Client
	apiURL string
	logger *slog.Logger
}

// PromptData contains data used to build article generation prompts.
type PromptData struct {
	Topic            string
	TopicDescription string
	Keywords         string
	Tone             string
	Length           string
	TargetAudience   string
	IncludeCode      bool
	PreviousTitles   []string
}

// NewGenerator creates a new article generator with the specified API key and configuration.
func NewGenerator(apiKey string, cfg *config.Config) Generator {
	timeout := time.Duration(cfg.AI.TimeoutSeconds) * time.Second
	logger := slog.Default().With("component", "article.generator")
	return &claudeGenerator{
		apiKey: apiKey,
		config: cfg,
		client: &http.Client{Timeout: timeout},
		apiURL: "https://api.anthropic.com/v1/messages",
		logger: logger,
	}
}

// NewGeneratorWithLogger creates a new article generator with a custom logger.
func NewGeneratorWithLogger(apiKey string, cfg *config.Config, logger *slog.Logger) Generator {
	timeout := time.Duration(cfg.AI.TimeoutSeconds) * time.Second
	return &claudeGenerator{
		apiKey: apiKey,
		config: cfg,
		client: &http.Client{Timeout: timeout},
		apiURL: "https://api.anthropic.com/v1/messages",
		logger: logger.With("component", "article.generator"),
	}
}

// Generate creates a new article with context support for cancellation.
func (g *claudeGenerator) Generate(ctx context.Context, topic string, history *storage.ArticleHistory) (*Article, error) {
	logger := g.logger.With(
		"topic", topic,
		"previous_articles_count", len(history.Articles),
	)
	logger.InfoContext(ctx, "Starting article generation")

	// Build context about previous articles
	previousTitles := []string{}
	for _, article := range history.Articles {
		if article.Topic == topic {
			previousTitles = append(previousTitles, article.Title)
		}
	}

	if len(previousTitles) > 0 {
		logger.InfoContext(ctx, "Found previous articles on this topic",
			"count", len(previousTitles),
			"titles", previousTitles)
	}

	// Get topic details
	topicDetails := g.config.GetTopicDetails(topic)
	if topicDetails != nil {
		logger.DebugContext(ctx, "Retrieved topic details",
			"description", topicDetails.Description,
			"keywords", topicDetails.Keywords)
	} else {
		logger.WarnContext(ctx, "No topic details found for topic")
	}

	// Build the prompt using template
	logger.DebugContext(ctx, "Building prompt from template")
	prompt := g.buildPromptFromTemplate(topic, topicDetails, previousTitles)

	// Get system prompt
	systemPrompt := g.getSystemPrompt()

	// Call Claude API with retry logic
	logger.InfoContext(ctx, "Calling Claude API",
		"model", g.config.AI.Model,
		"max_tokens", g.config.AI.MaxTokens)
	response, err := g.callClaudeAPIWithRetry(ctx, systemPrompt, prompt)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to call Claude API",
			"error", err)
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}

	// Parse the response
	logger.DebugContext(ctx, "Parsing Claude API response")
	article, err := g.parseResponse(response)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to parse Claude response",
			"error", err,
			"response_length", len(response))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	article.PublishedAt = time.Now()
	logger.InfoContext(ctx, "Successfully generated article",
		"title", article.Title,
		"content_length", len(article.Content),
		"tags", article.Tags)

	return article, nil
}

func (g *claudeGenerator) buildPromptFromTemplate(topic string, topicDetails *config.TopicConfig, previousTitles []string) string {
	// Load template
	templateContent, err := g.config.GetPromptTemplate()
	if err != nil {
		g.logger.Warn("Failed to load prompt template, falling back to built-in",
			"template_path", g.config.GetPromptTemplatePath(),
			"error", err)
		return g.buildPromptFallback(topic, topicDetails, previousTitles)
	}

	// Parse template
	tmpl, err := template.New("prompt").Parse(string(templateContent))
	if err != nil {
		g.logger.Warn("Failed to parse prompt template, falling back to built-in",
			"error", err)
		return g.buildPromptFallback(topic, topicDetails, previousTitles)
	}

	// Prepare data
	data := PromptData{
		Topic:          topic,
		Tone:           g.config.Style.Tone,
		Length:         g.config.Style.Length,
		TargetAudience: g.config.Style.TargetAudience,
		IncludeCode:    g.config.Style.IncludeCode,
		PreviousTitles: previousTitles,
	}

	if topicDetails != nil {
		data.TopicDescription = topicDetails.Description
		if len(topicDetails.Keywords) > 0 {
			data.Keywords = strings.Join(topicDetails.Keywords, ", ")
		}
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		g.logger.Warn("Failed to execute prompt template, falling back to built-in",
			"error", err)
		return g.buildPromptFallback(topic, topicDetails, previousTitles)
	}

	g.logger.Debug("Successfully built prompt from template",
		"prompt_length", buf.Len())
	return buf.String()
}

func (g *claudeGenerator) buildPromptFallback(topic string, topicDetails *config.TopicConfig, previousTitles []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are a technical writer creating an engaging article for Medium. ")
	prompt.WriteString(fmt.Sprintf("Write a %s article about: %s\n\n", g.config.Style.Length, topic))

	if topicDetails != nil && topicDetails.Description != "" {
		prompt.WriteString(fmt.Sprintf("Focus area: %s\n\n", topicDetails.Description))
		if len(topicDetails.Keywords) > 0 {
			prompt.WriteString(fmt.Sprintf("Include these concepts: %s\n\n", strings.Join(topicDetails.Keywords, ", ")))
		}
	}

	prompt.WriteString("Style requirements:\n")
	prompt.WriteString(fmt.Sprintf("- Tone: %s\n", g.config.Style.Tone))
	prompt.WriteString(fmt.Sprintf("- Target audience: %s\n", g.config.Style.TargetAudience))
	if g.config.Style.IncludeCode {
		prompt.WriteString("- Include practical code examples\n")
	}
	prompt.WriteString("\n")

	if len(previousTitles) > 0 {
		prompt.WriteString("Previously written articles on this topic (avoid duplicating):\n")
		for _, title := range previousTitles {
			prompt.WriteString(fmt.Sprintf("- %s\n", title))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Article requirements:\n")
	prompt.WriteString("1. Create a compelling, SEO-friendly title\n")
	prompt.WriteString("2. Write the article in Markdown format\n")
	prompt.WriteString("3. Include an engaging introduction\n")
	prompt.WriteString("4. Use proper headings (##, ###) for structure\n")
	prompt.WriteString("5. Add a conclusion with key takeaways\n")
	prompt.WriteString("6. Suggest 3-5 relevant tags\n\n")

	prompt.WriteString("Return your response in this JSON format:\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"title\": \"Article Title\",\n")
	prompt.WriteString("  \"content\": \"Full article content in Markdown...\",\n")
	prompt.WriteString("  \"tags\": [\"tag1\", \"tag2\", \"tag3\"]\n")
	prompt.WriteString("}\n")

	return prompt.String()
}

func (g *claudeGenerator) getSystemPrompt() string {
	content, err := g.config.GetSystemPrompt()
	if err != nil {
		// Use default system prompt on error
		return "You are an expert technical writer specializing in software engineering topics."
	}
	return string(content)
}

// callClaudeAPIWithRetry calls the Claude API with exponential backoff retry logic.
func (g *claudeGenerator) callClaudeAPIWithRetry(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := range maxRetries {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds (2s, 4s, 8s)
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			g.logger.InfoContext(ctx, "Retrying API call after backoff",
				"attempt", attempt+1,
				"max_attempts", maxRetries,
				"backoff_seconds", backoff.Seconds())

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				g.logger.WarnContext(ctx, "Context cancelled during retry backoff",
					"attempt", attempt+1)
				return "", ctx.Err()
			}
		}

		response, err := g.callClaudeAPI(ctx, systemPrompt, userPrompt)
		if err == nil {
			if attempt > 0 {
				g.logger.InfoContext(ctx, "API call succeeded after retry",
					"attempt", attempt+1)
			}
			return response, nil
		}

		lastErr = err

		// Check if error is retryable (5xx, rate limit, timeout)
		if !isRetryableError(err) {
			g.logger.WarnContext(ctx, "Non-retryable error encountered",
				"attempt", attempt+1,
				"error", err)
			return "", err
		}

		g.logger.WarnContext(ctx, "Retryable error encountered",
			"attempt", attempt+1,
			"error", err)
	}

	g.logger.ErrorContext(ctx, "Max retries exceeded",
		"max_attempts", maxRetries,
		"last_error", lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError determines if an error should be retried.
func isRetryableError(err error) bool {
	// Check for context errors (not retryable)
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	errStr := err.Error()
	// Retry on server errors, rate limits, and timeouts
	return strings.Contains(errStr, "status 5") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused")
}

func (g *claudeGenerator) callClaudeAPI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Get temperature value (default to 1.0 if nil)
	temperature := 1.0
	if g.config.AI.Temperature != nil {
		temperature = *g.config.AI.Temperature
	}

	requestBody := map[string]any{
		"model":       g.config.AI.Model,
		"max_tokens":  g.config.AI.MaxTokens,
		"temperature": temperature,
		"system":      systemPrompt,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": userPrompt,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		g.logger.ErrorContext(ctx, "Failed to marshal request body", "error", err)
		return "", err
	}

	g.logger.DebugContext(ctx, "Sending request to Claude API",
		"url", g.apiURL,
		"model", g.config.AI.Model,
		"max_tokens", g.config.AI.MaxTokens,
		"temperature", temperature)

	req, err := http.NewRequestWithContext(ctx, "POST", g.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		g.logger.ErrorContext(ctx, "Failed to create HTTP request", "error", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := g.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		g.logger.ErrorContext(ctx, "HTTP request failed",
			"error", err,
			"duration_ms", duration.Milliseconds())
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	g.logger.DebugContext(ctx, "Received response from Claude API",
		"status_code", resp.StatusCode,
		"duration_ms", duration.Milliseconds())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.ErrorContext(ctx, "Failed to read response body", "error", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		g.logger.ErrorContext(ctx, "API returned non-OK status",
			"status_code", resp.StatusCode,
			"response_body", string(body))
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		g.logger.ErrorContext(ctx, "Failed to unmarshal API response",
			"error", err,
			"response_body", string(body))
		return "", err
	}

	if len(response.Content) == 0 {
		g.logger.ErrorContext(ctx, "API response contains no content")
		return "", fmt.Errorf("no content in response")
	}

	g.logger.DebugContext(ctx, "Successfully received content from API",
		"response_length", len(response.Content[0].Text))

	return response.Content[0].Text, nil
}

func (g *claudeGenerator) parseResponse(response string) (*Article, error) {
	// Try to extract JSON from response (in case there's extra text)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[start : end+1]

	var result struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &Article{
		Title:   result.Title,
		Content: result.Content,
		Tags:    result.Tags,
	}, nil
}

var _ Generator = &claudeGenerator{}
