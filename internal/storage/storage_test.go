package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONStoreLoad(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		wantLen int
	}{
		{
			name: "valid history",
			json: `{
				"articles": [
					{
						"title": "Test Article",
						"topic": "Go",
						"published_at": "2025-01-01T00:00:00Z",
						"url": "https://medium.com/test",
						"tags": ["go", "testing"]
					}
				]
			}`,
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "empty history",
			json:    `{"articles": []}`,
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "invalid json",
			json:    `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			storePath := filepath.Join(tmpDir, "test.json")

			if tt.json != "" {
				if err := os.WriteFile(storePath, []byte(tt.json), 0600); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			store := NewJSONStore(storePath)
			history, err := store.Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(history.Articles) != tt.wantLen {
				t.Errorf("Load() articles len = %v, want %v", len(history.Articles), tt.wantLen)
			}
		})
	}
}

func TestJSONStoreLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "nonexistent.json")

	store := NewJSONStore(storePath)
	history, err := store.Load()

	if err != nil {
		t.Errorf("Load() should not error on non-existent file, got: %v", err)
	}

	if history == nil || len(history.Articles) != 0 {
		t.Error("Load() should return empty history for non-existent file")
	}
}

func TestJSONStoreSave(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test.json")

	store := NewJSONStore(storePath)

	history := &ArticleHistory{
		Articles: []ArticleRecord{
			{
				Title:       "Test Article",
				Topic:       "Go Programming",
				PublishedAt: time.Now(),
				URL:         "https://medium.com/test",
				Tags:        []string{"go", "programming"},
			},
		},
	}

	err := store.Save(history)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Error("Save() did not create file")
	}

	// Load and verify
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}

	if len(loaded.Articles) != 1 {
		t.Errorf("Load() after Save() articles len = %v, want 1", len(loaded.Articles))
	}

	if loaded.Articles[0].Title != history.Articles[0].Title {
		t.Errorf("Load() after Save() title = %v, want %v", loaded.Articles[0].Title, history.Articles[0].Title)
	}
}

func TestArticleRecord(t *testing.T) {
	now := time.Now()
	record := ArticleRecord{
		Title:       "Test",
		Topic:       "Testing",
		PublishedAt: now,
		URL:         "https://test.com",
		Tags:        []string{"test"},
	}

	if record.Title != "Test" {
		t.Errorf("Title = %v, want Test", record.Title)
	}

	if record.Topic != "Testing" {
		t.Errorf("Topic = %v, want Testing", record.Topic)
	}

	if !record.PublishedAt.Equal(now) {
		t.Errorf("PublishedAt mismatch")
	}
}
