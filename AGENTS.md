# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Overview

`clone-a-gnome` is a Go CLI tool that enables rapid cloning of remote PostgreSQL databases into local Docker containers. The tool streams data directly from `pg_dump` to `psql` without creating intermediate files, making it efficient for database cloning workflows.

## Common Commands

### Build and Development
```bash
# Build the binary
go build -o clone-a-gnome ./cmd/clone-a-gnome

# Build all packages (no tests exist yet)
go build ./...

# Run the tool
./clone-a-gnome postgresql://user:pass@host/dbname

# Clean build artifacts
rm -f clone-a-gnome
```

### Usage Examples
```bash
# Basic database clone
./clone-a-gnome postgresql://user:secret@ep-sample.eu-west-3.aws.neon.tech/mydb

# Clone with table exclusion
./clone-a-gnome <SOURCE_URL> --exclude-table sensitive_data --exclude-table logs

# Reuse existing container
./clone-a-gnome <SOURCE_URL> --reuse

# Run with anonymization script
./clone-a-gnome <SOURCE_URL> --anonymize-script anonymize.sql

# Debug mode
./clone-a-gnome <SOURCE_URL> --log-level debug
```

### Docker Management
```bash
# Stop the container (tool reminds you of this command)
docker stop clone-a-gnome

# Remove container to start fresh
docker rm clone-a-gnome

# Check container status
docker ps -a --filter name=clone-a-gnome
```

## Architecture

### Core Components

**CLI Layer** (`internal/cli/`):
- `root.go`: Cobra command definition with all CLI flags and options
- `run.go`: Main execution logic orchestrating all components

**Docker Management** (`internal/dockerx/`):
- Container lifecycle management (create, reuse, inspect)
- Automatic port allocation and PostgreSQL readiness checking
- Image management with `postgres:16-alpine` default

**PostgreSQL Operations** (`internal/postgres/`):
- `connection.go`: URL parsing and connection info validation
- `transfer.go`: Streaming data transfer via `pg_dump` | `docker exec psql`
- `password.go`: Secure password generation for local containers

**State Management** (`internal/state/`):
- Persistent storage at `~/.config/clone-a-gnome/state.json`
- Container reuse support with saved credentials and ports
- Thread-safe JSON-based state management

**Logging** (`internal/logging/`):
- Structured logging with configurable levels (debug, info, warn, error)
- Stream redirection for external process output

### Data Flow

1. **Validation**: Parse and validate source PostgreSQL URL
2. **Container Setup**: Create or reuse Docker container with postgres:16-alpine
3. **Prerequisites Check**: Verify `docker`, `pg_dump`, and `psql` availability
4. **Streaming Transfer**: Direct pipe from `pg_dump` stdout to `docker exec psql` stdin
5. **Post-Processing**: Optional SQL script execution for data anonymization
6. **State Persistence**: Save container details for reuse functionality

### Key Design Decisions

- **No intermediate files**: Streams data directly between processes for efficiency
- **Container reuse**: Maintains connection strings across runs via persistent state
- **French localization**: All user-facing messages and comments are in French
- **External dependency validation**: Fails fast if required tools are missing
- **Automatic port allocation**: Uses ephemeral ports to avoid conflicts

## Dependencies

**Runtime Requirements**:
- Docker CLI accessible in PATH
- PostgreSQL client tools (`pg_dump`, `psql`)
- Go 1.25+ for building

**Go Dependencies**:
- `github.com/spf13/cobra` v1.8.0 for CLI framework
- Standard library only (no external database drivers needed)

## Configuration

### State File Location
The tool stores persistent state at:
- macOS/Linux: `~/.config/clone-a-gnome/state.json`
- Fallback: `~/.clone-a-gnome/state.json`

### Supported PostgreSQL URLs
- `postgres://` and `postgresql://` schemes
- Full connection parameters including host, port, database, credentials
- Query parameters are preserved and passed to pg_dump

### Container Naming
- Default container name: `clone-a-gnome`
- Customizable via `--container-name` flag
- Must be unique within Docker environment unless using `--reuse`