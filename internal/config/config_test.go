package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
