// Package main provides the entry point for the autoblog-ai application.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/yourusername/autoblog-ai/internal/article"
	"github.com/yourusername/autoblog-ai/internal/config"
	"github.com/yourusername/autoblog-ai/internal/medium"
	"github.com/yourusername/autoblog-ai/internal/storage"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	dryRun := flag.Bool("dry-run", false, "Generate article but don't publish")
	topicFlag := flag.String("topic", "", "Specific topic to write about (overrides random selection)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get API keys from config (with env var override)
	anthropicKey := cfg.GetAnthropicKey()
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required (set in config.yaml or environment variable)")
	}

	mediumToken := cfg.GetMediumToken()
	if mediumToken == "" && !*dryRun {
		log.Fatal("MEDIUM_TOKEN is required (set in config.yaml or environment variable, or use --dry-run)")
	}

	// Initialize services
	generator := article.NewGenerator(anthropicKey, cfg)
	publisher := medium.NewPublisher(mediumToken)
	store := storage.NewJSONStore("articles.json")

	// Load article history
	history, err := store.Load()
	if err != nil {
		log.Printf("Warning: Could not load article history: %v", err)
		history = &storage.ArticleHistory{Articles: []storage.ArticleRecord{}}
	}

	// Select topic
	var topic string
	if *topicFlag != "" {
		topic = *topicFlag
	} else {
		topic = cfg.SelectRandomTopic()
	}

	log.Printf("Generating article about: %s", topic)

	// Generate article
	generatedArticle, err := generator.Generate(topic, history)
	if err != nil {
		log.Fatalf("Failed to generate article: %v", err)
	}

	log.Printf("Generated article: %s", generatedArticle.Title)
	log.Printf("Word count: %d", len(generatedArticle.Content)/5) // Rough estimate

	// Save article locally
	if err := saveArticleLocally(generatedArticle); err != nil {
		log.Printf("Warning: Could not save article locally: %v", err)
	}

	if *dryRun {
		log.Println("Dry run mode - article generated but not published")
		fmt.Println("\n--- ARTICLE PREVIEW ---")
		fmt.Printf("Title: %s\n", generatedArticle.Title)
		fmt.Printf("Tags: %v\n", generatedArticle.Tags)
		fmt.Printf("\n%s\n", generatedArticle.Content[:minInt(500, len(generatedArticle.Content))])
		fmt.Println("\n... (truncated)")
		return
	}

	// Publish to Medium
	log.Println("Publishing to Medium...")
	publishedURL, err := publisher.Publish(generatedArticle)
	if err != nil {
		log.Fatalf("Failed to publish article: %v", err)
	}

	log.Printf("Successfully published: %s", publishedURL)

	// Update history
	history.Articles = append(history.Articles, storage.ArticleRecord{
		Title:       generatedArticle.Title,
		Topic:       topic,
		PublishedAt: generatedArticle.PublishedAt,
		URL:         publishedURL,
		Tags:        generatedArticle.Tags,
	})

	if err := store.Save(history); err != nil {
		log.Printf("Warning: Could not save article history: %v", err)
	}

	log.Println("Done!")
}

func saveArticleLocally(article *article.Article) error {
	// #nosec G301 -- 0755 is appropriate for output directory
	if err := os.MkdirAll("generated", 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	filename := fmt.Sprintf("generated/%s.md", sanitizeFilename(article.Title))
	return os.WriteFile(filename, []byte(article.Content), 0600)
}

func sanitizeFilename(s string) string {
	// Simple sanitization - replace spaces and special chars
	result := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			result += string(c)
		} else if c == ' ' {
			result += "-"
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
