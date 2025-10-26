// Package storage provides persistence for article history.
package storage

import (
	"encoding/json"
	"os"
	"time"
)

// ArticleHistory contains a collection of published articles.
type ArticleHistory struct {
	Articles []ArticleRecord `json:"articles"`
}

// ArticleRecord represents a single published article.
type ArticleRecord struct {
	Title       string    `json:"title"`
	Topic       string    `json:"topic"`
	PublishedAt time.Time `json:"published_at"`
	URL         string    `json:"url"`
	Tags        []string  `json:"tags"`
}

// JSONStore manages article history persistence in JSON format.
type JSONStore struct {
	filepath string
}

// NewJSONStore creates a new JSON store at the specified file path.
func NewJSONStore(filepath string) *JSONStore {
	return &JSONStore{filepath: filepath}
}

// Load reads the article history from the JSON file.
func (s *JSONStore) Load() (*ArticleHistory, error) {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ArticleHistory{Articles: []ArticleRecord{}}, nil
		}
		return nil, err
	}

	var history ArticleHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return &history, nil
}

// Save writes the article history to the JSON file.
func (s *JSONStore) Save(history *ArticleHistory) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filepath, data, 0600)
}
