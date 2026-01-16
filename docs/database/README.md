# Docker MCP Gateway Database

## Overview

The Docker MCP Gateway uses an embedded SQLite database to store configuration data, profiles (working sets), server catalogs, and migration status. The database is located at `~/.docker/mcp/mcp-toolkit.db` and is automatically created and migrated when the CLI first runs.

This document covers the database architecture, migration system, concurrent access handling, and troubleshooting.

> 📘 **Deep Dive Available**: For detailed technical analysis of the migration race condition bug and why file locking is required for SQLite, see [MIGRATION_CONCURRENCY.md](./MIGRATION_CONCURRENCY.md)

## Architecture

### Database Location

```
~/.docker/mcp/
├── mcp-toolkit.db           # Main SQLite database
├── .mcp-toolkit-migration.lock  # Migration lock file (created during migrations)
└── catalogs/                # Catalog YAML files (legacy)
```

### Database Schema

The database schema is managed through versioned migrations in `pkg/db/migrations/`:

1. **001_initial.up.sql** - Initial schema with working_sets table
2. **002_create_catalogs.up.sql** - Catalogs table for server definitions
3. **003_add_catalog_last_update.up.sql** - Timestamp tracking for catalog updates
4. **004_create_migration_status.up.sql** - Migration status tracking
5. **005_add_remote_support.up.sql** - Remote server support

### Key Components

1. **Database Package** (`pkg/db/`)
   - DAO interface for all database operations
   - Migration management with golang-migrate
   - Connection pooling and configuration
   - File locking for concurrent access safety

2. **Working Sets (Profiles)** (`pkg/workingset/`)
   - JSON-serialized server configurations
   - Secret provider mappings
   - Server snapshots for reproducibility

3. **Catalog Management** (`pkg/catalog/`)
   - Server definitions and metadata
   - Last update timestamps
   - Catalog source tracking

4. **Migration Status** (`pkg/migrate/`)
   - Tracks migration history
   - Handles data migrations from YAML to database
   - Ensures migration idempotency

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
- **Timeout**: 30 seconds
- **Behavior**:
  - First process acquires lock and runs migrations
  - Other processes wait for lock to be released
  - All processes continue once migrations complete

### Why File Locking Is Necessary

Unlike PostgreSQL and MySQL, which use database-level advisory locks (`pg_advisory_lock()` and `GET_LOCK()`), **golang-migrate's SQLite lock is only in-memory** and doesn't coordinate between processes.

**The Problem**: When multiple processes start simultaneously:
1. All check the database state (clean)
2. All try to run migrations at the same time
3. Race condition occurs → "Dirty database version" error

**The Solution**: OS-level file locking ensures only one process runs migrations at a time. Other processes wait, then see migrations already complete.

SQLite's `busy_timeout` pragma helps with individual write conflicts, but doesn't prevent the migration race condition because the "check state → decide → migrate" sequence isn't atomic.

> 📖 **For detailed technical analysis** including code, timelines, and why other solutions don't work, see [MIGRATION_CONCURRENCY.md](./MIGRATION_CONCURRENCY.md)

### The Concurrent Migration Problem

**Scenario**: When the database doesn't exist (first run, corruption, or deletion) and multiple `docker mcp` commands start simultaneously:

```
Time  Process 1              Process 2              Process 3
----  -------------------    -------------------    -------------------
T0    Starts migration 1     Starts migration 1     Starts migration 1
T1    Sets dirty flag=true   Sees dirty flag=true   Sees dirty flag=true
T2    Running migration...   ❌ ErrDirty - FAIL     ❌ ErrDirty - FAIL
T3    Completes migration
T4    Sets dirty flag=false
```

**Without File Locking**: golang-migrate sets a "dirty" flag during migrations. When Process 1 starts migrating, Processes 2 and 3 immediately fail with:
```
failed to run migrations: Dirty database version 1. Fix and force version.
```

**With File Locking**: Only one process can migrate at a time:

```
Time  Process 1              Process 2              Process 3
----  -------------------    -------------------    -------------------
T0    Acquires lock ✓        Waiting for lock...   Waiting for lock...
T1    Starts migration 1
T2    Running migration...
T3    Completes migration
T4    Releases lock
T5                           Acquires lock ✓        Waiting for lock...
T6                           No migrations needed
T7                           Releases lock
T8                                                  Acquires lock ✓
T9                                                  No migrations needed
T10                                                 Releases lock
```

### Migration Safety Features

1. **File Locking**: OS-level lock prevents concurrent migrations
2. **Transactional Migrations**: Each migration runs in a transaction (rollback on failure)
3. **Version Tracking**: Database tracks current schema version
4. **Dirty State Detection**: Detects incomplete migrations
5. **Retry Logic**: Retries transient errors (e.g., database busy)
6. **Connection Limits**: Single connection limit prevents lock contention

## Configuration

### SQLite Driver

We use `modernc.org/sqlite`, a pure-Go SQLite implementation:
- CGo-free (no C dependencies)
- Standard `database/sql` interface
- Cross-platform support (Linux, macOS, Windows)

### Connection Settings

Database connection (in `pkg/db/db.go:72`):

```go
"file:"+dbFile+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
```

**Key Settings**:
- **busy_timeout=5000**: Wait up to 5 seconds for write locks (helps with contention)
- **foreign_keys=ON**: Enforce referential integrity
- **MaxOpenConns=1**: Single connection (SQLite has limited write concurrency)
- **MaxIdleConns=1**: Keep connection alive for reuse
- **ConnMaxLifetime=0**: Persistent connection

### File Locking Configuration

Migration file locking (in `pkg/db/db.go:100-122`):

```go
lockFile := filepath.Join(filepath.Dir(dbFile), ".mcp-toolkit-migration.lock")
fileLock := flock.New(lockFile)
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
```

- **Lock File**: `.mcp-toolkit-migration.lock` in same directory as database
- **Timeout**: 30 seconds maximum wait
- **Retry Interval**: Check every 100ms if lock is available
- **Platform Support**: Works on Linux, macOS, Windows

## Concurrent Access Patterns

### Read Operations

Multiple processes can read from the database simultaneously:
- SQLite allows unlimited concurrent readers
- No locking required for read-only operations
- Examples: `profile list`, `catalog list`, `server list`

### Write Operations

Write operations use SQLite's built-in locking:
- Writers acquire exclusive locks automatically
- `busy_timeout` allows retries on lock contention
- Examples: `profile create`, `catalog add`, `server add`

### Migration Operations

Migrations use both SQLite locks AND file locks:
- File lock prevents multiple processes from attempting migrations
- SQLite locks protect individual migration transactions
- This two-level locking ensures safety during initialization

## Performance Characteristics

### Database Size

Typical database size: 100-500 KB
- Working sets: ~1-10 KB each (JSON serialized)
- Catalogs: ~5-20 KB each
- Migration status: <1 KB

### Query Performance

All operations are optimized for local file access:
- Reads: <1ms typical
- Writes: <5ms typical
- Migrations: <100ms for full migration set
- Lock acquisition: <100ms typical, <30s worst case

### Memory Usage

Minimal memory footprint:
- Single connection pool
- No query caching required
- Embedded in CLI process
- No separate database server

## Testing

### Concurrent Migration Test

Test that file locking prevents migration race conditions:

```bash
go run test_migration_race.go
```

This test launches 100 concurrent processes that all try to initialize the database simultaneously. With file locking, all 100 should succeed (100% success rate). Without file locking, most would fail with "Dirty database version" errors.

> 📖 **For detailed test results and analysis**, see the [Testing section in MIGRATION_CONCURRENCY.md](./MIGRATION_CONCURRENCY.md#testing-the-fix)

### Manual Testing

Test migration behavior manually:

```bash
# Remove database to force migration
rm ~/.docker/mcp/mcp-toolkit.db

# Run a command that requires the database
docker mcp profile list

# Verify database was created
ls -lh ~/.docker/mcp/mcp-toolkit.db

# Check that lock file was cleaned up
ls ~/.docker/mcp/.mcp-toolkit-migration.lock  # Should not exist
```

### Testing Concurrent Access

Test multiple processes accessing the database:

```bash
# Terminal 1: Long-running process
docker mcp gateway run --transport sse --port 3000

# Terminal 2: Concurrent command
docker mcp profile list

# Terminal 3: Another concurrent command
docker mcp catalog list

# All should work without conflicts
```

## Troubleshooting

### "Dirty database version N" Error

**Symptom**:
```
failed to run migrations: Dirty database version 1. Fix and force version.
```

**Cause**: A migration started but didn't complete (process killed, system crash, or race condition in older versions)

**Solution**:
1. **Check if migration lock exists**:
   ```bash
   ls ~/.docker/mcp/.mcp-toolkit-migration.lock
   ```
   If it exists, another process is currently migrating. Wait for it to complete.

2. **Check database state**:
   ```bash
   sqlite3 ~/.docker/mcp/mcp-toolkit.db "SELECT * FROM schema_migrations;"
   ```

3. **Force fix** (if migrations are stuck):
   ```bash
   # Backup first
   cp ~/.docker/mcp/mcp-toolkit.db ~/.docker/mcp/mcp-toolkit.db.backup

   # Option A: Delete and recreate
   rm ~/.docker/mcp/mcp-toolkit.db
   docker mcp profile list  # Will recreate database

   # Option B: Manual fix (advanced)
   sqlite3 ~/.docker/mcp/mcp-toolkit.db "UPDATE schema_migrations SET dirty=0 WHERE version=1;"
   ```

### "Database is locked" Error

**Symptom**:
```
failed to open database: database is locked
```

**Cause**: Another process has an exclusive lock on the database

**Solution**:
```bash
# Check for processes using the database
lsof ~/.docker/mcp/mcp-toolkit.db

# If a gateway is running, stop it
pkill -f "docker mcp gateway"

# Retry the operation
docker mcp profile list
```

### "Timeout waiting for migration lock" Error

**Symptom**:
```
failed to acquire migration lock: context deadline exceeded
```

**Cause**: Migration lock held for >30 seconds (very slow migration or stuck process)

**Solution**:
```bash
# Check for stale lock file
ls -l ~/.docker/mcp/.mcp-toolkit-migration.lock

# Check if a process is actually running migrations
ps aux | grep "docker mcp"

# If no processes but lock exists, remove it
rm ~/.docker/mcp/.mcp-toolkit-migration.lock

# Retry
docker mcp profile list
```

### Database Corruption

**Symptom**:
```
failed to open database: file is not a database
```

**Cause**: Database file corrupted (incomplete write, system crash, disk failure)

**Solution**:
```bash
# Backup corrupted database (for investigation)
mv ~/.docker/mcp/mcp-toolkit.db ~/.docker/mcp/mcp-toolkit.db.corrupted

# Recreate database (will be empty)
docker mcp profile list

# Note: Previous profiles and catalog data will be lost
# Check YAML backups: ~/.docker/mcp/catalogs/ and registry.yaml
```

### Performance Issues

**Symptom**: Slow database operations

**Diagnostics**:
```bash
# Check database size
ls -lh ~/.docker/mcp/mcp-toolkit.db

# Check for database fragmentation
sqlite3 ~/.docker/mcp/mcp-toolkit.db "PRAGMA integrity_check;"

# Check for lock contention
lsof ~/.docker/mcp/mcp-toolkit.db
```

**Solution**:
```bash
# Vacuum database (reclaim space, reduce fragmentation)
sqlite3 ~/.docker/mcp/mcp-toolkit.db "VACUUM;"

# Analyze for query optimization
sqlite3 ~/.docker/mcp/mcp-toolkit.db "ANALYZE;"
```

## Migration History and Data Evolution

### From YAML to Database (Migration 004)

The MCP Gateway originally stored configuration in YAML files:
- `~/.docker/mcp/registry.yaml` - Enabled servers
- `~/.docker/mcp/config.yaml` - Server configurations
- `~/.docker/mcp/tools.yaml` - Tool selections

Migration 004 (`pkg/migrate/migrate.go`) automatically migrates this data:
1. Reads existing YAML files
2. Creates a "default" profile with all configured servers
3. Preserves server configurations and tool selections
4. Keeps YAML files as backup

**Note**: After migration, the CLI uses the database. YAML files are not updated but can be used for recovery.

### Future Migrations

New migrations should:
1. Be idempotent (safe to run multiple times)
2. Preserve existing data
3. Use transactions for atomicity
4. Include both `.up.sql` and `.down.sql` files
5. Test with concurrent execution

## Development Guidelines

### Adding New Migrations

1. **Create migration files**:
   ```bash
   cd pkg/db/migrations
   # Use next version number
   touch 006_your_migration.up.sql
   touch 006_your_migration.down.sql
   ```

2. **Write SQL**:
   ```sql
   -- 006_your_migration.up.sql
   CREATE TABLE IF NOT EXISTS your_table (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       data TEXT NOT NULL
   );
   ```

3. **Test migration**:
   ```bash
   # Delete database to test fresh migration
   rm ~/.docker/mcp/mcp-toolkit.db

   # Run command to trigger migration
   docker mcp profile list

   # Verify schema
   sqlite3 ~/.docker/mcp/mcp-toolkit.db ".schema"
   ```

4. **Test concurrent execution**:
   ```bash
   cd docs/database/testing
   go run test_migration_race.go
   ```

### DAO Pattern

All database access uses the DAO (Data Access Object) pattern:

```go
// Define interface
type MyDAO interface {
    CreateThing(ctx context.Context, thing Thing) error
    GetThing(ctx context.Context, id int) (Thing, error)
}

// Implement in pkg/db/
func (d *dao) CreateThing(ctx context.Context, thing Thing) error {
    _, err := d.db.ExecContext(ctx, "INSERT INTO things ...")
    return err
}
```

Benefits:
- Easy to test with mocks
- Clear separation of concerns
- Type-safe database operations

### Testing Database Code

```go
// Use in-memory database for tests
func TestMyDAO(t *testing.T) {
    dao, err := db.New(db.WithDatabaseFile(":memory:"))
    require.NoError(t, err)
    defer dao.Close()

    // Test operations
    err = dao.CreateThing(ctx, thing)
    assert.NoError(t, err)
}
```

## Platform Compatibility

The database system works identically across all supported platforms:

### Linux
- Database: `~/.docker/mcp/mcp-toolkit.db`
- Lock file: Uses `flock()` syscall
- Works on all Linux distributions

### macOS
- Database: `~/.docker/mcp/mcp-toolkit.db`
- Lock file: Uses `flock()` syscall
- Works on all macOS versions

### Windows
- Database: `%USERPROFILE%\.docker\mcp\mcp-toolkit.db`
- Lock file: Uses `LockFileEx()` Win32 API
- Works on Windows 10+

The `gofrs/flock` library handles platform differences automatically.

## Security Considerations

### File Permissions

The database and lock files are created with appropriate permissions:
- Database: `0644` (user read/write, others read)
- Directory: `0755` (user full, others read/execute)
- Lock file: `0644` (user read/write, others read)

### Sensitive Data

The database may contain sensitive information:
- OAuth tokens (encrypted)
- API keys (encrypted)
- Server configurations
- Secret provider mappings

**Important**: Never commit `mcp-toolkit.db` to version control.

### Multi-User Systems

Each user has their own database in their home directory:
- No cross-user access
- No privilege escalation risks
- No shared lock files

## Backup and Recovery

### Automatic Backups

The CLI doesn't automatically backup the database. Consider:

```bash
# Manual backup
cp ~/.docker/mcp/mcp-toolkit.db ~/.docker/mcp/mcp-toolkit.db.backup

# Automated backup (add to cron/launchd)
#!/bin/bash
BACKUP_DIR=~/.docker/mcp/backups
mkdir -p $BACKUP_DIR
DATE=$(date +%Y%m%d_%H%M%S)
cp ~/.docker/mcp/mcp-toolkit.db $BACKUP_DIR/mcp-toolkit-$DATE.db
```

### Recovery from Backup

```bash
# Stop any running gateways
pkill -f "docker mcp gateway"

# Restore from backup
cp ~/.docker/mcp/mcp-toolkit.db.backup ~/.docker/mcp/mcp-toolkit.db

# Verify
docker mcp profile list
```

### Exporting Data

```bash
# Export to SQL
sqlite3 ~/.docker/mcp/mcp-toolkit.db .dump > mcp-toolkit.sql

# Export specific table
sqlite3 ~/.docker/mcp/mcp-toolkit.db "SELECT * FROM working_sets;" > working_sets.csv
```

## Performance Tuning

### For High-Frequency Operations

If experiencing performance issues:

```bash
# Enable WAL mode for better concurrency
sqlite3 ~/.docker/mcp/mcp-toolkit.db "PRAGMA journal_mode=WAL;"

# Increase cache size (default is 2MB)
sqlite3 ~/.docker/mcp/mcp-toolkit.db "PRAGMA cache_size=10000;"  # 10MB
```

### For Large Databases

If database grows large (>100MB):

```bash
# Analyze for query optimization
sqlite3 ~/.docker/mcp/mcp-toolkit.db "ANALYZE;"

# Vacuum to reclaim space
sqlite3 ~/.docker/mcp/mcp-toolkit.db "VACUUM;"
```

## References

### Documentation

- **[MIGRATION_CONCURRENCY.md](./MIGRATION_CONCURRENCY.md)** - Detailed technical analysis of the migration race condition bug, including code examples, timelines, and why various solutions don't work
- [SQLite Documentation](https://www.sqlite.org/docs.html) - Official SQLite docs
- [golang-migrate](https://github.com/golang-migrate/migrate) - Database migration tool
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure-Go SQLite driver
- [gofrs/flock](https://github.com/gofrs/flock) - Cross-platform file locking library

### Related

- [MCP Gateway Repository](https://github.com/docker/mcp-gateway) - Source code
- [Database Package](https://github.com/docker/mcp-gateway/tree/main/pkg/db) - DAO implementation
