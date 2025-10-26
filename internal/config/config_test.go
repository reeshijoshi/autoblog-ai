package config

import (
	"os"
	"path/filepath"
	"testing"
)

const defaultFallbackTopic = "Software Engineering Best Practices"

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid config",
			yaml: `
ai:
  model: "claude-sonnet-4-20250514"
  max_tokens: 8192
  temperature: 1.0
  timeout_seconds: 120
style:
  tone: "professional"
  length: "medium"
  target_audience: "intermediate"
  include_code: true
topics:
  - name: "Test Topic"
    weight: 1
`,
			wantErr: false,
		},
		{
			name: "minimal config with defaults",
			yaml: `
style:
  tone: "casual"
topics:
  - name: "Test Topic"
    weight: 1
`,
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			yaml:    `invalid: [yaml`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file and directories
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			// Create template files needed for validation
			templatesDir := filepath.Join(tmpDir, "templates")
			// #nosec G301 -- test directory permissions are acceptable
			if err := os.MkdirAll(templatesDir, 0755); err != nil {
				t.Fatalf("Failed to create templates dir: %v", err)
			}
			articlePromptPath := filepath.Join(templatesDir, "article-prompt.md")
			systemPromptPath := filepath.Join(templatesDir, "system-prompt.md")
			if err := os.WriteFile(articlePromptPath, []byte("test prompt"), 0600); err != nil {
				t.Fatalf("Failed to write article prompt: %v", err)
			}
			if err := os.WriteFile(systemPromptPath, []byte("test system"), 0600); err != nil {
				t.Fatalf("Failed to write system prompt: %v", err)
			}

			// Update YAML to use correct paths
			yamlWithPaths := tt.yaml + `
prompt_template: ` + articlePromptPath + `
system_prompt: ` + systemPromptPath + `
`

			if err := os.WriteFile(configPath, []byte(yamlWithPaths), 0600); err != nil {
				t.Fatalf("Failed to write temp config: %v", err)
			}

			cfg, err := Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check defaults are applied
				if cfg.AI.Model == "" {
					t.Error("AI model should have default value")
				}
				if cfg.Style.Tone == "" {
					t.Error("Style tone should have default value")
				}
			}
		})
	}
}

func TestGetAnthropicKey(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		config string
		want   string
	}{
		{
			name:   "env var takes precedence",
			envVar: "env-key",
			config: "config-key",
			want:   "env-key",
		},
		{
			name:   "use config when no env var",
			envVar: "",
			config: "config-key",
			want:   "config-key",
		},
		{
			name:   "empty when both empty",
			envVar: "",
			config: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env var
			if tt.envVar != "" {
				_ = os.Setenv("ANTHROPIC_API_KEY", tt.envVar)
				defer func() { _ = os.Unsetenv("ANTHROPIC_API_KEY") }()
			} else {
				_ = os.Unsetenv("ANTHROPIC_API_KEY")
			}

			cfg := &Config{
				APIKeys: APIKeysConfig{
					Anthropic: tt.config,
				},
			}

			got := cfg.GetAnthropicKey()
			if got != tt.want {
				t.Errorf("GetAnthropicKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMediumToken(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		config string
		want   string
	}{
		{
			name:   "env var takes precedence",
			envVar: "env-token",
			config: "config-token",
			want:   "env-token",
		},
		{
			name:   "use config when no env var",
			envVar: "",
			config: "config-token",
			want:   "config-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				_ = os.Setenv("MEDIUM_TOKEN", tt.envVar)
				defer func() { _ = os.Unsetenv("MEDIUM_TOKEN") }()
			} else {
				_ = os.Unsetenv("MEDIUM_TOKEN")
			}

			cfg := &Config{
				APIKeys: APIKeysConfig{
					Medium: tt.config,
				},
			}

			got := cfg.GetMediumToken()
			if got != tt.want {
				t.Errorf("GetMediumToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectRandomTopic(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{
			{Name: "Topic 1", Weight: 1},
			{Name: "Topic 2", Weight: 2},
			{Name: "Topic 3", Weight: 3},
		},
	}

	// Run multiple times to ensure it returns valid topics
	topicCounts := make(map[string]int)
	for i := 0; i < 100; i++ {
		topic := cfg.SelectRandomTopic()
		topicCounts[topic]++

		// Verify it's a valid topic
		found := false
		for _, t := range cfg.Topics {
			if t.Name == topic {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SelectRandomTopic() returned invalid topic: %s", topic)
		}
	}

	// Verify all topics were selected at least once (probabilistic)
	if len(topicCounts) < 2 {
		t.Error("SelectRandomTopic() should select different topics")
	}
}

func TestGetTopicDetails(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{
			{
				Name:        "Go Concurrency",
				Description: "Advanced patterns",
				Keywords:    []string{"goroutines", "channels"},
				Weight:      3,
			},
		},
	}

	tests := []struct {
		name      string
		topicName string
		wantNil   bool
	}{
		{
			name:      "existing topic",
			topicName: "Go Concurrency",
			wantNil:   false,
		},
		{
			name:      "non-existing topic",
			topicName: "Invalid Topic",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetTopicDetails(tt.topicName)
			if (got == nil) != tt.wantNil {
				t.Errorf("GetTopicDetails() nil = %v, wantNil %v", got == nil, tt.wantNil)
			}

			if !tt.wantNil && got.Name != tt.topicName {
				t.Errorf("GetTopicDetails() name = %v, want %v", got.Name, tt.topicName)
			}
		})
	}
}

func TestLoadTopicsFromCSV(t *testing.T) {
	tests := []struct {
		name    string
		csv     string
		wantLen int
		wantErr bool
	}{
		{
			name: "valid CSV",
			csv: `name,description,keywords,weight
"Topic 1","Description 1","key1,key2",3
"Topic 2","Description 2","key3,key4",2`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "missing header",
			csv:     `"Topic 1","Description 1","key1,key2",3`,
			wantErr: true,
		},
		{
			name:    "empty file",
			csv:     ``,
			wantErr: true,
		},
		{
			name: "with quotes in values",
			csv: `name,description,keywords,weight
"Topic with ""quotes""","Description","key1,key2",1`,
			wantLen: 1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			csvPath := filepath.Join(tmpDir, "topics.csv")
			if err := os.WriteFile(csvPath, []byte(tt.csv), 0600); err != nil {
				t.Fatalf("Failed to write temp CSV: %v", err)
			}

			topics, err := loadTopicsFromCSV(csvPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadTopicsFromCSV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(topics) != tt.wantLen {
				t.Errorf("loadTopicsFromCSV() len = %v, want %v", len(topics), tt.wantLen)
			}
		})
	}
}

func TestGetPromptTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "prompt.md")
	expectedContent := "Test prompt template"
	if err := os.WriteFile(templatePath, []byte(expectedContent), 0600); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	cfg := &Config{
		PromptTemplate: templatePath,
	}

	content, err := cfg.GetPromptTemplate()
	if err != nil {
		t.Errorf("GetPromptTemplate() error = %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("GetPromptTemplate() = %v, want %v", string(content), expectedContent)
	}

	// Test with non-existent file
	cfg2 := &Config{
		PromptTemplate: "/nonexistent/path.md",
	}
	_, err = cfg2.GetPromptTemplate()
	if err == nil {
		t.Error("GetPromptTemplate() should error on non-existent file")
	}
}

func TestGetSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "system.md")
	expectedContent := "You are a technical writer"
	if err := os.WriteFile(promptPath, []byte(expectedContent), 0600); err != nil {
		t.Fatalf("Failed to write system prompt: %v", err)
	}

	cfg := &Config{
		SystemPrompt: promptPath,
	}

	content, err := cfg.GetSystemPrompt()
	if err != nil {
		t.Errorf("GetSystemPrompt() error = %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("GetSystemPrompt() = %v, want %v", string(content), expectedContent)
	}
}

func TestGetPromptTemplatePath(t *testing.T) {
	cfg := &Config{
		PromptTemplate: "/path/to/template.md",
	}

	path := cfg.GetPromptTemplatePath()
	if path != "/path/to/template.md" {
		t.Errorf("GetPromptTemplatePath() = %v, want /path/to/template.md", path)
	}
}

func TestGetSystemPromptPath(t *testing.T) {
	cfg := &Config{
		SystemPrompt: "/path/to/system.md",
	}

	path := cfg.GetSystemPromptPath()
	if path != "/path/to/system.md" {
		t.Errorf("GetSystemPromptPath() = %v, want /path/to/system.md", path)
	}
}

func TestValidate_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "missing topics",
			cfg: &Config{
				Topics: []TopicConfig{},
			},
			wantErr: true,
		},
		{
			name: "invalid topic weight",
			cfg: &Config{
				Topics: []TopicConfig{
					{Name: "Test", Weight: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "missing prompt template file",
			cfg: &Config{
				Topics:         []TopicConfig{{Name: "Test", Weight: 1}},
				PromptTemplate: "/nonexistent/path.md",
			},
			wantErr: true,
		},
		{
			name: "missing system prompt file",
			cfg: &Config{
				Topics:       []TopicConfig{{Name: "Test", Weight: 1}},
				SystemPrompt: "/nonexistent/path.md",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSelectRandomTopic_EmptyTopics(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{},
	}

	topic := cfg.SelectRandomTopic()
	// When topics is empty, it returns a default fallback topic
	if topic != defaultFallbackTopic {
		t.Errorf("SelectRandomTopic() with empty topics = %v, want %q", topic, defaultFallbackTopic)
	}
}

func TestSelectRandomTopic_SingleTopic(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{
			{Name: "Only Topic", Weight: 1},
		},
	}

	// Run multiple times to ensure it always returns the same topic
	for i := 0; i < 10; i++ {
		topic := cfg.SelectRandomTopic()
		if topic != "Only Topic" {
			t.Errorf("SelectRandomTopic() = %v, want 'Only Topic'", topic)
		}
	}
}

func TestExportTopicsToCSV(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{
			{
				Name:        "Go Programming",
				Description: "Go best practices",
				Keywords:    []string{"golang", "concurrency"},
				Weight:      3,
			},
			{
				Name:        "Testing",
				Description: "Testing strategies",
				Keywords:    []string{"unit testing", "integration"},
				Weight:      2,
			},
		},
	}

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "export", "topics.csv")

	err := cfg.ExportTopicsToCSV(csvPath)
	if err != nil {
		t.Fatalf("ExportTopicsToCSV() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Error("CSV file was not created")
	}

	// Read back and verify content
	// #nosec G304 -- csvPath is a test-controlled file path
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("Failed to read exported CSV: %v", err)
	}

	content := string(data)
	expectedStrings := []string{
		"name",
		"description",
		"keywords",
		"weight",
		"Go Programming",
		"Testing",
	}

	for _, expected := range expectedStrings {
		if !contains(content, expected) {
			t.Errorf("Exported CSV missing expected content: %s", expected)
		}
	}
}

func TestExportTopicsToCSV_ErrorCases(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{
			{Name: "Test", Weight: 1},
		},
	}

	// Test with invalid path (read-only location)
	err := cfg.ExportTopicsToCSV("/nonexistent/readonly/path.csv")
	if err == nil {
		t.Error("ExportTopicsToCSV() should error with invalid path")
	}
}

func TestLoad_WithTopicsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CSV topics file
	csvPath := filepath.Join(tmpDir, "topics.csv")
	csvContent := `name,description,keywords,weight
"CSV Topic 1","CSV Description 1","key1,key2",5
"CSV Topic 2","CSV Description 2","key3",3`
	if err := os.WriteFile(csvPath, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}

	// Create template files
	templatesDir := filepath.Join(tmpDir, "templates")
	// #nosec G301 -- test directory permissions are acceptable
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	articlePromptPath := filepath.Join(templatesDir, "article-prompt.md")
	systemPromptPath := filepath.Join(templatesDir, "system-prompt.md")
	if err := os.WriteFile(articlePromptPath, []byte("test prompt"), 0600); err != nil {
		t.Fatalf("Failed to write article prompt: %v", err)
	}
	if err := os.WriteFile(systemPromptPath, []byte("test system"), 0600); err != nil {
		t.Fatalf("Failed to write system prompt: %v", err)
	}

	// Create config file that references the CSV
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
topics_file: ` + csvPath + `
prompt_template: ` + articlePromptPath + `
system_prompt: ` + systemPromptPath
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Topics) != 2 {
		t.Errorf("Topics length = %v, want 2 (from CSV)", len(cfg.Topics))
	}

	if cfg.Topics[0].Name != "CSV Topic 1" {
		t.Errorf("First topic name = %v, want 'CSV Topic 1'", cfg.Topics[0].Name)
	}
}

func TestLoad_NoTopicsUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()

	// Create template files
	templatesDir := filepath.Join(tmpDir, "templates")
	// #nosec G301 -- test directory permissions are acceptable
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	articlePromptPath := filepath.Join(templatesDir, "article-prompt.md")
	systemPromptPath := filepath.Join(templatesDir, "system-prompt.md")
	if err := os.WriteFile(articlePromptPath, []byte("test prompt"), 0600); err != nil {
		t.Fatalf("Failed to write article prompt: %v", err)
	}
	if err := os.WriteFile(systemPromptPath, []byte("test system"), 0600); err != nil {
		t.Fatalf("Failed to write system prompt: %v", err)
	}

	// Config with no topics - should use defaults
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
prompt_template: ` + articlePromptPath + `
system_prompt: ` + systemPromptPath
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Topics) == 0 {
		t.Error("Should have default topics when none specified")
	}

	// Should contain default topic
	if cfg.Topics[0].Name != defaultFallbackTopic {
		t.Errorf("Default topic = %v, want %q", cfg.Topics[0].Name, defaultFallbackTopic)
	}
}

func TestLoad_InvalidTopicsCSV(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid CSV file
	csvPath := filepath.Join(tmpDir, "invalid.csv")
	if err := os.WriteFile(csvPath, []byte("invalid csv content"), 0600); err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `topics_file: ` + csvPath
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() should return error for invalid CSV file")
	}

	if !contains(err.Error(), "failed to load topics from CSV") {
		t.Errorf("Error should mention CSV loading, got: %v", err)
	}
}

func TestValidate_MaxTokensOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	// Create required template files
	promptPath := filepath.Join(tmpDir, "prompt.md")
	systemPath := filepath.Join(tmpDir, "system.md")
	_ = os.WriteFile(promptPath, []byte("test"), 0600)
	_ = os.WriteFile(systemPath, []byte("test"), 0600)

	tests := []struct {
		name      string
		maxTokens int
		wantErr   bool
	}{
		{"valid lower bound", 1, false},
		{"valid upper bound", 200000, false},
		{"valid middle", 8192, false},
		{"too low", 0, true},
		{"too high", 200001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AI: AIConfig{
					Model:          "test-model",
					MaxTokens:      tt.maxTokens,
					TimeoutSeconds: 60,
				},
				Topics:         []TopicConfig{{Name: "Test", Weight: 1}},
				PromptTemplate: promptPath,
				SystemPrompt:   systemPath,
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadTopicsFromCSV_InvalidWeight(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "topics.csv")
	csvContent := `name,description,keywords,weight
"Topic 1","Description","keywords",invalid`
	if err := os.WriteFile(csvPath, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}

	topics, err := loadTopicsFromCSV(csvPath)
	if err != nil {
		t.Fatalf("loadTopicsFromCSV() unexpected error = %v", err)
	}

	// Invalid weight should default to 1
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic, got %d", len(topics))
	}

	if topics[0].Weight != 1 {
		t.Errorf("Invalid weight should default to 1, got %d", topics[0].Weight)
	}
}

func TestSelectRandomTopic_WeightedDistribution(t *testing.T) {
	cfg := &Config{
		Topics: []TopicConfig{
			{Name: "Topic A", Weight: 10},
			{Name: "Topic B", Weight: 1},
		},
	}

	// Run many times to check distribution (Topic A should appear more often)
	counts := make(map[string]int)
	iterations := 1000
	for i := 0; i < iterations; i++ {
		topic := cfg.SelectRandomTopic()
		counts[topic]++
	}

	// Topic A should have been selected (roughly 10x more than Topic B)
	// Allow for randomness, but it should be significantly more
	if counts["Topic A"] < 500 {
		t.Errorf("Topic A count = %d, expected > 500 (weighted 10x higher)", counts["Topic A"])
	}

	if counts["Topic B"] > 500 {
		t.Errorf("Topic B count = %d, expected < 500 (weighted 10x lower)", counts["Topic B"])
	}
}

// Helper function for string containment check
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
