# AutoBlog AI

Automated technical article generator and publisher using Claude AI. Generates articles on configurable topics and publishes them to Medium via GitHub Actions.

## Quick Start

```bash
# 1. Clone and install
git clone https://github.com/yourusername/autoblog-ai.git
cd autoblog-ai
go mod download

# 2. Set up API keys (create .env file)
cp .env.example .env
# Edit .env with your ANTHROPIC_API_KEY and MEDIUM_TOKEN

# 3. Customize topics
# Edit topics.csv with your preferred topics

# 4. Test locally
go run main.go --dry-run

# 5. Publish
go run main.go
```

## Requirements

- **Go 1.25.3+**
- **Anthropic API Key** - Get from [console.anthropic.com](https://console.anthropic.com)
- **Medium Token** - Get from [Medium Settings > Integration tokens](https://medium.com/me/settings)

## Configuration

Edit `config.yaml`:

```yaml
ai:
  model: "claude-sonnet-4-20250514"
  max_tokens: 8192
  temperature: 1.0
  timeout_seconds: 120

style:
  tone: "professional"              # professional, casual, technical, conversational
  length: "medium"                  # short (800-1200), medium (1500-2500), long (3000+)
  target_audience: "intermediate"   # beginners, intermediate, advanced
  include_code: true
```

Edit `topics.csv` (supports Excel/Google Sheets):

```csv
name,description,keywords,weight
"Your Topic","What to cover","keyword1,keyword2,keyword3",3
"Another Topic","Focus area","keyword4,keyword5",2
```

## Usage

```bash
# Run with random topic
go run main.go

# Dry run (preview without publishing)
go run main.go --dry-run

# Specific topic
go run main.go --topic "Advanced Go Concurrency"

# Custom config
go run main.go --config custom.yaml
```

## GitHub Actions Setup

1. **Add secrets** in GitHub repo: Settings > Secrets and variables > Actions
   - `ANTHROPIC_API_KEY` - Your Anthropic API key
   - `MEDIUM_TOKEN` - Your Medium integration token

2. **Configure schedule** in `.github/workflows/publish-article.yml`:
   ```yaml
   on:
     schedule:
       - cron: '0 9 * * 1'  # Every Monday at 9 AM UTC
   ```

3. **Manual trigger**: Actions tab > Auto-Generate and Publish Article > Run workflow

## Development

```bash
# Run all checks (format, lint, test)
make check

# Run tests
make test

# Build binary
make build

# Hot reload during development
make dev

# See all commands
make help
```

## Docker

```bash
# Build and run
docker build -t autoblog-ai .
docker run --env-file .env autoblog-ai --dry-run

# Or use docker-compose
docker-compose up
```

## Kubernetes/Helm

```bash
# Deploy to Kubernetes
kubectl apply -f k8s/

# Or use Helm
helm install autoblog-ai ./helm/autoblog-ai
```

## Project Structure

```
autoblog-ai/
├── main.go                        # Entry point
├── config.yaml                    # AI & style config
├── topics.csv                     # Topics list (editable in Excel)
├── templates/                     # Prompt templates
│   ├── article-prompt.md
│   └── system-prompt.md
├── internal/
│   ├── article/generator.go      # Claude API integration
│   ├── config/config.go           # Config management
│   ├── medium/publisher.go       # Medium API
│   └── storage/storage.go        # History tracking
├── .github/workflows/             # GitHub Actions
└── generated/                     # Output articles
```

## Troubleshooting

**Articles are too similar**: Add more diverse topics in `topics.csv`, increase keyword variety

**API rate limits**: Reduce schedule frequency, built-in retry logic handles transient errors

**Publishing fails**: Verify Medium token is valid and has write permissions

**Config errors**: Run with `--dry-run` to validate config before publishing

## License

Non-Commercial Individual Use License - see [LICENSE](LICENSE)

**TL;DR**: Free for individuals to use however they want (including personal monetization). Companies/businesses need a commercial license.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run `make check` to verify
5. Submit a pull request
