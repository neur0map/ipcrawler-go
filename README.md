# IPCrawler

A Go CLI tool for crawling IP addresses and domains.

## Installation

```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/ipcrawler.git
cd ipcrawler

# Initial setup
./scripts/setup.sh
# OR
make install
```

## Usage

```bash
# Basic usage
ipcrawler 192.168.1.1
ipcrawler example.com

# With options
ipcrawler --debug example.com
ipcrawler --workflow custom.yaml 8.8.8.8
ipcrawler --version
ipcrawler --health
```

## Development

```bash
# Build
make

# Update (pull latest + rebuild)
make update

# Auto-rebuild on changes
make dev

# Run without building
make run ARGS='192.168.1.1 --debug'
```

## Updating

To update to the latest version:

```bash
# From anywhere
make update

# Or use the update script
./scripts/update.sh
```

The update command will:
- Pull the latest changes from git
- Handle uncommitted changes safely
- Rebuild the binary
- Update your global installation

## Project Structure

```
ipcrawler/
├── main.go              # Entry point
├── cmd/
│   └── cli.go          # CLI logic
├── workflows/
│   └── default.yaml    # Default workflow
├── scripts/            # Helper scripts
├── Makefile           # Build automation
└── README.md
```