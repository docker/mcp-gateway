# Docker MCP Gateway Database

## Overview

The Docker MCP Gateway uses an embedded SQLite database to store configuration data, profiles (working sets), server catalogs, and migration status. The database is located at `~/.docker/mcp/mcp-toolkit.db` and is automatically created and migrated when the CLI first runs.

This document covers the database architecture, migration system, concurrent access handling, and troubleshooting.

## Architecture

### Database Location

```
~/.docker/mcp/
├── mcp-toolkit.db           # Main SQLite database
├── .mcp-toolkit-migration.lock  # Migration lock file (persists after creation)
```

### Database Schema

The database schema is managed through versioned migrations in `pkg/db/migrations/`

### Key Components

**Database Package** (`pkg/db/`)
   - DAO interface for all database operations
   - Migration management with golang-migrate
   - Connection pooling and configuration
   - File locking for concurrent access safety


## Migration System

### How Migrations Work

The Docker MCP Gateway uses [golang-migrate](https://github.com/golang-migrate/migrate) for schema versioning:

```
Application Startup
    ↓
pkg/db/New() called
    ↓
Check for pending migrations
    ↓
[Acquire File Lock] ← Prevents concurrent migrations
    ↓
Run migrations sequentially
    ↓
[Release File Lock]
    ↓
Database ready for use
```

### Migration Lock File

To prevent race conditions when multiple processes start simultaneously, migrations use an OS-level file lock:

- **Lock File**: `~/.docker/mcp/.mcp-toolkit-migration.lock`
- **Library**: `github.com/gofrs/flock` (cross-platform)
- **Persistence**: The lock file persists on disk after migrations complete (this is intentional and safe)

### Migration Operations

Migrations use both SQLite locks AND file locks:
- File lock prevents multiple processes from attempting migrations
- SQLite locks protect individual migration transactions
- This two-level locking ensures safety during initialization

### Why File Locking Is Necessary

Migrations with [golang-migrate](https://github.com/golang-migrate/migrate) are not run within a single transaction. There is the possibility
for a race condition if two process concurrently attempt to run the migrations.

**The Problem**: When multiple processes start simultaneously:
1. All check the database state (clean)
2. All try to run migrations at the same time
3. Race condition occurs → "Dirty database version" error

**The Solution**: OS-level file locking ensures only one process runs migrations at a time. Other processes wait, then see migrations already complete.


## Testing

### Concurrent Migration Test

Test that file locking prevents migration race conditions:

```bash
go run test_migration_race.go
```

This test launches 100 concurrent processes that all try to initialize the database simultaneously. With file locking, all 100 should succeed (100% success rate). Without file locking, most would fail with "Dirty database version" errors.