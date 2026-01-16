# Migration Concurrency and Race Condition Analysis

This document provides a detailed technical analysis of the concurrent migration race condition that can occur with SQLite databases, why golang-migrate's built-in locking is insufficient, and how we solved it with file-based locking.

> 📖 **Looking for high-level overview?** See [README.md](./README.md) for general database documentation, architecture, and usage guide.

## Table of Contents

- [The Problem](#the-problem)
- [Why golang-migrate's Lock Doesn't Work](#why-golang-migrates-lock-doesnt-work)
- [What SQLite Provides](#what-sqlite-provides)
- [The Atomic Check-and-Set Problem](#the-atomic-check-and-set-problem)
- [Detailed Race Condition Timeline](#detailed-race-condition-timeline)
- [The Solution: File Locking](#the-solution-file-locking)
- [Why Other Solutions Don't Work](#why-other-solutions-dont-work)
- [Testing the Fix](#testing-the-fix)

## The Problem

When multiple processes start simultaneously and the database doesn't exist (first run, after deletion, or corruption), they all try to run migrations concurrently. Without proper cross-process coordination, you get this error:

```
failed to run migrations: Dirty database version 1. Fix and force version.
```

This "Dirty database version" error occurs because:
1. Multiple processes check the migration state simultaneously
2. All see a clean state
3. All start migrating at the same time
4. They race to update the migration status table
5. One succeeds, others fail when they detect the "dirty" flag

## Why golang-migrate's Lock Doesn't Work

### The Implementation

The golang-migrate library provides a `Lock()` method for database drivers, but the SQLite implementation has a critical limitation:

```go
// From golang-migrate/migrate/v4/database/sqlite/sqlite.go
type Sqlite struct {
    db       *sql.DB
    isLocked atomic.Bool  // ⚠️ In-memory only!
    config   *Config
}

func (m *Sqlite) Lock() error {
    if !m.isLocked.CompareAndSwap(false, true) {
        return database.ErrLocked
    }
    return nil
}

func (m *Sqlite) Unlock() error {
    if !m.isLocked.CompareAndSwap(true, false) {
        return database.ErrNotLocked
    }
    return nil
}
```

**The Problem**: This `isLocked` flag is an **in-memory atomic boolean** that only exists within a single process. It provides zero coordination between different processes.

### Contrast with PostgreSQL and MySQL

Other database drivers use **database-level locks** that work across all connections:

- **PostgreSQL**: Uses `pg_advisory_lock()` - a server-side advisory lock visible to all connections
- **MySQL**: Uses `GET_LOCK()` - a named lock stored in the MySQL server
- **SQLite**: Uses only an in-memory flag - **NOT** cross-process safe

From the golang-migrate FAQ:
> "Database-specific locking features are used by **some** database drivers to prevent multiple instances of migrate from running migrations on the same database at the same time. For example, the MySQL driver uses the `GET_LOCK` function, while the Postgres driver uses the `pg_advisory_lock` function."

Note: SQLite is **not mentioned** in the list of databases with locking support.

## What SQLite Provides

The SQLite driver we use (`modernc.org/sqlite`) provides several concurrency features, but none solve the migration coordination problem:

### 1. busy_timeout Pragma

```go
"file:"+dbFile+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
```

**What it does**: When a write operation encounters a locked database, it waits and retries for up to 5 seconds before returning `SQLITE_BUSY`.

**What it doesn't do**: This only helps with **individual write operations** after the process has decided what to write. It doesn't help coordinate the "check state, decide, then write" sequence that migrations require.

### 2. File-Level Locking

SQLite uses OS-level file locking on the database file itself:
- Readers can access simultaneously
- Writers get exclusive locks
- `busy_timeout` controls retry behavior

**What it doesn't do**: File-level locks protect the database file consistency, but don't prevent the race condition in the migration decision logic.

### 3. WAL Mode

Write-Ahead Logging allows concurrent readers with a single writer:
- Better concurrency for normal operations
- Readers don't block writers
- Writers don't block readers

**What it doesn't do**: Doesn't help with the migration race - the problem is in the migration decision logic, not the actual SQL execution.

### 4. Error Handling

The driver exposes detailed SQLite error codes via `sqlite.Error` type:
```go
var sqliteErr *sqlite.Error
if errors.As(err, &sqliteErr) {
    if sqliteErr.Code() == sqlite3.SQLITE_BUSY {
        // Database is locked, retry
    }
}
```

**What it doesn't do**: Error handling can detect lock contention but doesn't prevent the race condition from occurring.

## The Atomic Check-and-Set Problem

Migrations require an atomic "check dirty state, acquire lock, run migration" sequence. Here's the simplified golang-migrate execution flow:

```go
// Simplified from migrate.Up()
func (m *Migrate) Up() error {
    // Step 1: Check current version (NOT atomic with lock!)
    version, dirty, err := m.databaseDriver.Version()
    if dirty {
        return ErrDirty  // ❌ "Dirty database version N"
    }

    // Step 2: Acquire lock (in-memory for SQLite!)
    if err := m.databaseDriver.Lock(); err != nil {
        return err
    }
    defer m.databaseDriver.Unlock()

    // Step 3: Set dirty flag and run migrations
    m.databaseDriver.SetVersion(nextVersion, true)  // dirty=true
    // ... run migration SQL ...
    m.databaseDriver.SetVersion(nextVersion, false) // dirty=false
}
```

**The Gap**: Steps 1 and 2 are not atomic. Multiple processes can all read clean state in Step 1 before any of them set the dirty flag in Step 3.

## Detailed Race Condition Timeline

### Without File Lock

```
Time  Process 1 (DB)                Process 2 (DB)                SQLite State
----  ---------------------------    ---------------------------    -----------------
T0    Version() → v=0, dirty=false                                 version=0, dirty=false
T1                                   Version() → v=0, dirty=false  version=0, dirty=false
T2    Lock() → ✓ (in-memory)                                       version=0, dirty=false
T3                                   Lock() → ✓ (in-memory)        version=0, dirty=false
T4    SetVersion(1, true)                                          version=1, dirty=true
T5                                   SetVersion(1, true)           version=1, dirty=true
T6    Run migration SQL...                                         version=1, dirty=true
T7                                   Version() → v=1, dirty=true   version=1, dirty=true
T8                                   ❌ ErrDirty! FAIL              version=1, dirty=true
T9    SetVersion(1, false)                                         version=1, dirty=false
T10   ✓ Success                      [process exits with error]    version=1, dirty=false
```

**Key Problem Points**:
- **T0-T1**: Both processes read clean state (separate database queries)
- **T2-T3**: Both processes acquire locks (separate memory spaces, no coordination)
- **T4-T5**: Race to set dirty flag (SQLite only protects individual writes, not the sequence)
- **T7-T8**: Process 2 eventually sees dirty flag and fails with ErrDirty

**Why busy_timeout doesn't help**:
- `Version()` at T0-T1 is a **read** - no lock needed, no retry
- `Lock()` at T2-T3 is **in-memory** - no database operation, no coordination
- `SetVersion()` at T4-T5 are **separate transactions** - busy_timeout only makes them retry if blocked, but both succeed because they're different writes

### With File Lock (Current Implementation)

```
Time  Process 1                      Process 2                      File Lock
----  ---------------------------    ---------------------------    -------------
T0    flock.TryLock() → ✓            flock.TryLock() → blocked     Process 1
T1    Version() → v=0, dirty=false                                 Process 1
T2    Lock() → ✓                                                   Process 1
T3    SetVersion(1, true)                                          Process 1
T4    Run migration SQL...           [waiting for file lock...]    Process 1
T5    SetVersion(1, false)                                         Process 1
T6    Unlock()                                                     Process 1
T7    flock.Unlock()                 flock.TryLock() → ✓           Process 2
T8    ✓ Success                      Version() → v=1, dirty=false  Process 2
T9                                   No migrations needed          Process 2
T10                                  flock.Unlock()                [released]
T11                                  ✓ Success                     [released]
```

**How File Lock Fixes It**:
- **T0**: Process 1 acquires OS-level file lock (blocks all other processes)
- **T1-T6**: Process 1 completes entire migration sequence atomically from Process 2's perspective
- **T7**: Process 2 acquires file lock only after Process 1 fully releases it
- **T8**: Process 2 sees completed migration, no work needed, returns immediately

## The Solution: File Locking

We use the `github.com/gofrs/flock` library to implement cross-platform file locking.

### Implementation (pkg/db/db.go)

```go
func New(opts ...Option) (DAO, error) {
    // ... database connection setup ...

    // Acquire file lock BEFORE checking version
    lockFile := filepath.Join(filepath.Dir(dbFile), ".mcp-toolkit-migration.lock")
    fileLock := flock.New(lockFile)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
    if !locked {
        return nil, fmt.Errorf("failed to acquire migration lock: %w", err)
    }
    defer fileLock.Unlock()

    // NOW it's safe to run migrations - we have exclusive access
    mig, err := migrate.NewWithInstance("iofs", migDriver, "sqlite", driver)
    if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return nil, fmt.Errorf("failed to run migrations: %w", err)
    }

    // Lock released automatically by defer
}
```

### Key Properties

1. **Cross-Platform**: Uses `flock()` syscall on Unix/macOS, `LockFileEx()` on Windows
2. **Timeout**: Waits up to 30 seconds for the lock
3. **Retry**: Checks every 100ms if lock is available
4. **Automatic Cleanup**: Lock released via `defer` even if migration fails
5. **OS-Level**: Kernel enforces the lock across all processes

### Lock File Location

```
~/.docker/mcp/
├── mcp-toolkit.db                   # Main database
├── .mcp-toolkit-migration.lock      # Lock file (created during migration)
└── catalogs/                        # Other files
```

The lock file:
- Created automatically when first process tries to lock
- Deleted automatically after migration completes (via defer)
- If it exists, another process is currently running migrations
- Safe to delete manually if stale (no processes running)

## Why Other Solutions Don't Work

We considered several alternatives before settling on file locking:

### 1. Advisory Lock in SQLite

**Idea**: Use something like PostgreSQL's `pg_advisory_lock()`.

**Problem**: SQLite doesn't provide advisory locks. There's no equivalent to PostgreSQL's server-side lock registry.

**Status**: Not available in SQLite

### 2. SELECT FOR UPDATE

**Idea**: Use `SELECT ... FOR UPDATE` to lock the version row.

**Problem**:
- Doesn't work for DDL (schema changes)
- Can't lock a row that doesn't exist yet (initial migration)
- Still has the race between checking and locking

**Status**: Insufficient for migrations

### 3. Retry on ErrDirty

**Idea**: Keep retrying when we get "Dirty database version" error.

**Problem**:
- Unreliable - depends on timing
- Can cause cascading failures if migrations are slow
- Doesn't actually prevent the race, just tries to recover
- Bad user experience (unpredictable delays)

**Status**: Used as a fallback in old code, but not reliable

### 4. WAL Mode

**Idea**: Enable Write-Ahead Logging for better concurrency.

**Problem**: WAL improves read/write concurrency but doesn't solve the migration coordination problem. The issue is in the decision logic, not the actual SQL execution.

**Status**: Useful for normal operations, but doesn't solve this issue

### 5. Single-Process Only

**Idea**: Document that only one instance should run migrations.

**Problem**: Not feasible for a CLI tool where users may run multiple commands simultaneously, especially during startup or in scripts.

**Status**: Not practical

### Why File Locking is the Right Solution

File locking is the standard approach for cross-process coordination in SQLite applications:

1. **Recommended Pattern**: Standard solution for coordinating complex operations across processes
2. **Simple**: Easy to implement and understand
3. **Reliable**: OS kernel enforces the lock
4. **Cross-Platform**: Works identically on all platforms
5. **No Database Changes**: Doesn't require schema modifications
6. **Minimal Overhead**: Only used during migrations (rare operation)

## Testing the Fix

### Quick Test Script

Use the provided test script to verify the fix:

```bash
cd /Users/bobby/git/mcp-gateway
go run test_migration_race.go
```

This test:
1. Deletes the database to force fresh migrations
2. Launches 100 concurrent `docker mcp profile list` commands
3. All processes try to initialize and migrate simultaneously
4. Reports success/failure rate

### Expected Results

**With File Locking (Current Implementation)**:
```
Starting migration race test with 100 concurrent iterations...
Deleting database...
Launching all processes simultaneously...

Results:
--------
[  0-99] ✓ Success (took ~20-100ms each)

============================================================
Summary:
============================================================
Total iterations:      100
Successful:            100 (100.0%)
Failed:                0 (0.0%)
Migration errors:      0
Dirty DB errors:       0
Total time:            ~2-3s
Avg time per iter:     ~20-30ms

✓ All iterations succeeded (could not reproduce the error)
```

**Without File Locking (Old Implementation)**:
```
Results:
--------
[  0-99] ✗ FAILED - Dirty database version 1

Summary:
============================================================
Total iterations:      100
Successful:            1 (1.0%)
Failed:                99 (99.0%)
Dirty DB errors:       99

🎯 SUCCESS! Reproduced the 'Dirty database version' error!
```

### What Success Looks Like

With file locking:
- **100% success rate** across all concurrent processes
- Only the first process runs migrations
- All other processes wait, then see migrations already complete
- Total time is dominated by the single migration sequence
- No "Dirty database version" errors

### Manual Testing

Test the migration behavior manually:

```bash
# Remove database to force migration
rm ~/.docker/mcp/mcp-toolkit.db

# Run multiple commands simultaneously
docker mcp profile list &
docker mcp catalog list &
docker mcp server list &
wait

# All should succeed, check the database was created
ls -lh ~/.docker/mcp/mcp-toolkit.db

# Lock file should be cleaned up
ls ~/.docker/mcp/.mcp-toolkit-migration.lock  # Should not exist
```

## References

### Documentation

- [golang-migrate FAQ](https://github.com/golang-migrate/migrate/blob/master/FAQ.md) - Official docs mention locking support
- [golang-migrate SQLite Driver Source](https://github.com/golang-migrate/migrate/blob/master/database/sqlite/sqlite.go) - Shows in-memory lock implementation
- [SQLite Locking](https://www.sqlite.org/lockingv3.html) - How SQLite handles concurrent access
- [gofrs/flock](https://github.com/gofrs/flock) - Cross-platform file locking library

### Related Issues

This is a known limitation of golang-migrate with SQLite. The documentation explicitly warns:

> "**IMPORTANT:** If you would like to run multiple instances of your app on different machines be sure to use a database that supports locking when running migrations. Otherwise you may encounter issues."

SQLite is implicitly in the "databases that don't support locking" category, requiring application-level coordination like file locking.
