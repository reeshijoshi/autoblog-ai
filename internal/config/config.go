// Package config provides configuration management for the autoblog-ai application.
package config

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main application configuration.
type Config struct {
	APIKeys        APIKeysConfig `yaml:"api_keys"`
	AI             AIConfig      `yaml:"ai"`
	Topics         []TopicConfig `yaml:"topics"`
	Style          StyleConfig   `yaml:"style"`
	TopicsFile     string        `yaml:"topics_file"`     // Optional: Path to CSV file
	PromptTemplate string        `yaml:"prompt_template"` // Optional: Path to prompt template
	SystemPrompt   string        `yaml:"system_prompt"`   // Optional: Path to system prompt
}

// APIKeysConfig contains API credentials for external services.
type APIKeysConfig struct {
	Anthropic string `yaml:"anthropic"` // Anthropic API key
	Medium    string `yaml:"medium"`    // Medium integration token
}

// AIConfig configures AI model parameters.
type AIConfig struct {
	Model          string   `yaml:"model"`           // Claude model to use
	MaxTokens      int      `yaml:"max_tokens"`      // Maximum tokens for generation
	Temperature    *float64 `yaml:"temperature"`     // Creativity level (0.0-1.0), pointer to distinguish unset from 0
	TimeoutSeconds int      `yaml:"timeout_seconds"` // API timeout in seconds
}

// TopicConfig defines a content topic with associated metadata.
type TopicConfig struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Keywords    []string `yaml:"keywords"`
	Weight      int      `yaml:"weight"` // Higher weight = more likely to be selected
}

// StyleConfig defines the writing style and format preferences.
type StyleConfig struct {
	Tone           string `yaml:"tone"`            // e.g., "professional", "casual", "technical"
	Length         string `yaml:"length"`          // e.g., "short", "medium", "long"
	TargetAudience string `yaml:"target_audience"` // e.g., "beginners", "intermediate", "advanced"
	IncludeCode    bool   `yaml:"include_code"`    // Whether to include code examples
}

// Load reads and parses a configuration file from the specified path.
func Load(path string) (*Config, error) {
	// #nosec G304 -- path is provided by user as configuration file path
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults for AI
	if config.AI.Model == "" {
		config.AI.Model = "claude-sonnet-4-20250514"
	}
	if config.AI.MaxTokens == 0 {
		config.AI.MaxTokens = 8192
	}
	if config.AI.Temperature == nil {
		defaultTemp := 1.0
		config.AI.Temperature = &defaultTemp
	}
	if config.AI.TimeoutSeconds == 0 {
		config.AI.TimeoutSeconds = 120
	}

	// Set defaults for style
	if config.Style.Tone == "" {
		config.Style.Tone = "professional"
	}
	if config.Style.Length == "" {
		config.Style.Length = "medium"
	}
	if config.Style.TargetAudience == "" {
		config.Style.TargetAudience = "intermediate"
	}

	// Set defaults for file paths
	if config.PromptTemplate == "" {
		config.PromptTemplate = "templates/article-prompt.md"
	}
	if config.SystemPrompt == "" {
		config.SystemPrompt = "templates/system-prompt.md"
	}

	// If topics file is specified, load from CSV
	if config.TopicsFile != "" {
		csvTopics, err := loadTopicsFromCSV(config.TopicsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load topics from CSV: %w", err)
		}
		// Replace or append topics from CSV
		if len(csvTopics) > 0 {
			config.Topics = csvTopics
		}
	}

	// If no topics loaded, use default
	if len(config.Topics) == 0 {
		config.Topics = getDefaultTopics()
	}

	// Override with environment variables (env vars take precedence)
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		config.APIKeys.Anthropic = anthropicKey
	}
	if mediumToken := os.Getenv("MEDIUM_TOKEN"); mediumToken != "" {
		config.APIKeys.Medium = mediumToken
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate AI settings
	if c.AI.MaxTokens < 1 || c.AI.MaxTokens > 200000 {
		return fmt.Errorf("ai.max_tokens must be between 1 and 200000, got %d", c.AI.MaxTokens)
	}
	if c.AI.Temperature != nil && (*c.AI.Temperature < 0 || *c.AI.Temperature > 1.0) {
		return fmt.Errorf("ai.temperature must be between 0.0 and 1.0, got %.2f", *c.AI.Temperature)
	}
	if c.AI.TimeoutSeconds < 1 || c.AI.TimeoutSeconds > 600 {
		return fmt.Errorf("ai.timeout_seconds must be between 1 and 600, got %d", c.AI.TimeoutSeconds)
	}
	if c.AI.Model == "" {
		return fmt.Errorf("ai.model cannot be empty")
	}

	// Validate file paths exist
	if _, err := os.Stat(c.PromptTemplate); err != nil {
		return fmt.Errorf("prompt_template file not found: %s", c.PromptTemplate)
	}
	if _, err := os.Stat(c.SystemPrompt); err != nil {
		return fmt.Errorf("system_prompt file not found: %s", c.SystemPrompt)
	}
	if c.TopicsFile != "" {
		if _, err := os.Stat(c.TopicsFile); err != nil {
			return fmt.Errorf("topics_file not found: %s", c.TopicsFile)
		}
	}

	// Validate topics
	if len(c.Topics) == 0 {
		return fmt.Errorf("at least one topic must be configured")
	}
	for i, topic := range c.Topics {
		if topic.Name == "" {
			return fmt.Errorf("topic %d has empty name", i)
		}
		if topic.Weight < 0 {
			return fmt.Errorf("topic %q has negative weight: %d", topic.Name, topic.Weight)
		}
	}

	return nil
}

// GetAnthropicKey returns the Anthropic API key with env var priority
func (c *Config) GetAnthropicKey() string {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	return c.APIKeys.Anthropic
}

// GetMediumToken returns the Medium token with env var priority
func (c *Config) GetMediumToken() string {
	if token := os.Getenv("MEDIUM_TOKEN"); token != "" {
		return token
	}
	return c.APIKeys.Medium
}

func loadTopicsFromCSV(path string) ([]TopicConfig, error) {
	// #nosec G304 -- path is from config file, user-controlled
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have header and at least one data row")
	}

	// Parse header to find column indices
	header := records[0]
	nameIdx, descIdx, keywordsIdx, weightIdx := -1, -1, -1, -1
	for i, col := range header {
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "name":
			nameIdx = i
		case "description":
			descIdx = i
		case "keywords":
			keywordsIdx = i
		case "weight":
			weightIdx = i
		}
	}

	if nameIdx == -1 {
		return nil, fmt.Errorf("CSV must have 'name' column")
	}

	topics := make([]TopicConfig, 0, len(records)-1)
	for i, record := range records[1:] {
		if len(record) <= nameIdx {
			continue
		}

		topic := TopicConfig{
			Name:   strings.TrimSpace(record[nameIdx]),
			Weight: 1, // Default weight
		}

		if descIdx != -1 && len(record) > descIdx {
			topic.Description = strings.TrimSpace(record[descIdx])
		}

		if keywordsIdx != -1 && len(record) > keywordsIdx {
			keywordsStr := strings.TrimSpace(record[keywordsIdx])
			if keywordsStr != "" {
				keywords := strings.Split(keywordsStr, ",")
				for _, kw := range keywords {
					if kw = strings.TrimSpace(kw); kw != "" {
						topic.Keywords = append(topic.Keywords, kw)
					}
				}
			}
		}

		if weightIdx != -1 && len(record) > weightIdx {
			if weight, err := strconv.Atoi(strings.TrimSpace(record[weightIdx])); err == nil {
				topic.Weight = weight
			}
		}

		if topic.Name == "" {
			fmt.Printf("Warning: Skipping row %d with empty name\n", i+2)
			continue
		}

		topics = append(topics, topic)
	}

	return topics, nil
}

// SelectRandomTopic chooses a random topic based on weights.
func (c *Config) SelectRandomTopic() string {
	if len(c.Topics) == 0 {
		return "Software Engineering Best Practices"
	}

	// Weighted random selection
	totalWeight := 0
	for _, topic := range c.Topics {
		weight := topic.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	// #nosec G404 -- crypto/rand not needed for topic selection
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	random := r.Intn(totalWeight)

	current := 0
	for _, topic := range c.Topics {
		weight := topic.Weight
		if weight <= 0 {
			weight = 1
		}
		current += weight
		if random < current {
			return topic.Name
		}
	}

	return c.Topics[0].Name
}

// GetTopicDetails returns the configuration for a specific topic by name.
func (c *Config) GetTopicDetails(name string) *TopicConfig {
	for _, topic := range c.Topics {
		if topic.Name == name {
			return &topic
		}
	}
	return nil
}

// GetPromptTemplate reads the prompt template file.
func (c *Config) GetPromptTemplate() ([]byte, error) {
	return os.ReadFile(c.PromptTemplate)
}

// GetSystemPrompt reads the system prompt file.
func (c *Config) GetSystemPrompt() ([]byte, error) {
	return os.ReadFile(c.SystemPrompt)
}

// GetPromptTemplatePath returns the path to the prompt template file.
func (c *Config) GetPromptTemplatePath() string {
	return c.PromptTemplate
}

// GetSystemPromptPath returns the path to the system prompt file.
func (c *Config) GetSystemPromptPath() string {
	return c.SystemPrompt
}

func getDefaultTopics() []TopicConfig {
	return []TopicConfig{
		{
			Name:        "Software Engineering Best Practices",
			Description: "General best practices in software development",
			Keywords:    []string{"clean code", "testing", "architecture"},
			Weight:      1,
		},
	}
}

// ExportTopicsToCSV exports current topics to a CSV file
func (c *Config) ExportTopicsToCSV(path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	// #nosec G301 -- 0755 is appropriate for output directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// #nosec G304 -- path is user-provided output file path
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"name", "description", "keywords", "weight"}); err != nil {
		return err
	}

	// Write topics
	for _, topic := range c.Topics {
		keywords := strings.Join(topic.Keywords, ",")
		weight := strconv.Itoa(topic.Weight)
		if err := writer.Write([]string{topic.Name, topic.Description, keywords, weight}); err != nil {
			return err
		}
	}

	return nil
}
