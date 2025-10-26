# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

# What's Changed in v0.2.1

## Version Bump: patch

### 🐛 Bug Fixes
- fix: release workflow permissions (9bbf6cc)
- fix: remove .github from ignored paths for release workflow (1385931)
- fix: release workflows and create CHANGELOG.md (912c3e3)

## Installation

Download the appropriate binary for your platform:
- **Linux (amd64)**: `autoblog-ai-linux-amd64.tar.gz`
- **Linux (arm64)**: `autoblog-ai-linux-arm64.tar.gz`
- **macOS (Intel)**: `autoblog-ai-darwin-amd64.tar.gz`
- **macOS (Apple Silicon)**: `autoblog-ai-darwin-arm64.tar.gz`
- **Windows**: `autoblog-ai-windows-amd64.zip`

## Quick Start

1. Extract the archive
2. Create a `.env` file with your API keys:
   ```
   ANTHROPIC_API_KEY=your_key
   MEDIUM_TOKEN=your_token
   ```
3. Run: `./autoblog-ai --dry-run`

See [README.md](https://github.com/reeshijoshi/autoblog-ai) for full documentation.

**Full Changelog**: https://github.com/reeshijoshi/autoblog-ai/compare/v0.2.0...v0.2.1
