// Package article provides functionality for generating articles using AI.
package article

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Generator handles article generation using Claude API.
type Generator struct {
	apiKey string
	config *config.Config
	client *http.Client
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
func NewGenerator(apiKey string, cfg *config.Config) *Generator {
	timeout := time.Duration(cfg.AI.TimeoutSeconds) * time.Second
	return &Generator{
		apiKey: apiKey,
		config: cfg,
		client: &http.Client{Timeout: timeout},
	}
}

// Generate creates a new article on the given topic, avoiding duplication based on article history.
func (g *Generator) Generate(topic string, history *storage.ArticleHistory) (*Article, error) {
	return g.GenerateWithContext(context.Background(), topic, history)
}

// GenerateWithContext creates a new article with context support for cancellation.
func (g *Generator) GenerateWithContext(ctx context.Context, topic string, history *storage.ArticleHistory) (*Article, error) {
	// Build context about previous articles
	previousTitles := []string{}
	for _, article := range history.Articles {
		if article.Topic == topic {
			previousTitles = append(previousTitles, article.Title)
		}
	}

	// Get topic details
	topicDetails := g.config.GetTopicDetails(topic)

	// Build the prompt using template
	prompt := g.buildPromptFromTemplate(topic, topicDetails, previousTitles)

	// Get system prompt
	systemPrompt := g.getSystemPrompt()

	// Call Claude API with retry logic
	response, err := g.callClaudeAPIWithRetry(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}

	// Parse the response
	article, err := g.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	article.PublishedAt = time.Now()
	return article, nil
}

func (g *Generator) buildPromptFromTemplate(topic string, topicDetails *config.TopicConfig, previousTitles []string) string {
	// Load template
	templateContent, err := g.config.GetPromptTemplate()
	if err != nil {
		fmt.Printf("Warning: Failed to load prompt template from %s: %v\n", g.config.GetPromptTemplatePath(), err)
		fmt.Println("Falling back to built-in template")
		return g.buildPromptFallback(topic, topicDetails, previousTitles)
	}

	// Parse template
	tmpl, err := template.New("prompt").Parse(string(templateContent))
	if err != nil {
		fmt.Printf("Warning: Failed to parse prompt template: %v\n", err)
		fmt.Println("Falling back to built-in template")
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
		fmt.Printf("Warning: Failed to execute prompt template: %v\n", err)
		fmt.Println("Falling back to built-in template")
		return g.buildPromptFallback(topic, topicDetails, previousTitles)
	}

	return buf.String()
}

func (g *Generator) buildPromptFallback(topic string, topicDetails *config.TopicConfig, previousTitles []string) string {
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

func (g *Generator) getSystemPrompt() string {
	content, err := g.config.GetSystemPrompt()
	if err != nil {
		// Use default system prompt on error
		return "You are an expert technical writer specializing in software engineering topics."
	}
	return string(content)
}

// callClaudeAPIWithRetry calls the Claude API with exponential backoff retry logic.
func (g *Generator) callClaudeAPIWithRetry(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds (2s, 4s, 8s)
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		response, err := g.callClaudeAPI(ctx, systemPrompt, userPrompt)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if error is retryable (5xx, rate limit, timeout)
		if !isRetryableError(err) {
			return "", err
		}
	}

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

func (g *Generator) callClaudeAPI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Get temperature value (default to 1.0 if nil)
	temperature := 1.0
	if g.config.AI.Temperature != nil {
		temperature = *g.config.AI.Temperature
	}

	requestBody := map[string]interface{}{
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
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.client.Do(req)
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

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return response.Content[0].Text, nil
}

func (g *Generator) parseResponse(response string) (*Article, error) {
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
