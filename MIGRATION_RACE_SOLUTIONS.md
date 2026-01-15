# Migration Race Condition - Three Solutions

This document describes three different solutions to fix the database migration race condition discovered during concurrent testing.

## Problem Summary

When 100 concurrent `docker mcp profile list` commands run simultaneously after deleting the database, **100% of them fail** with:
```
failed to run migrations: Dirty database version 1. Fix and force version.
```

**Root Cause:** golang-migrate sets a "dirty" flag when migrations start. When multiple processes race to initialize the database, they all see the dirty state and fail. The existing retry logic doesn't handle `ErrDirty` errors.

## Solutions Implemented

Each solution is in a separate git worktree for independent testing.

### Solution A: OS-Level File Locking (Recommended)
**Location:** `../mcp-gateway-solution-a`
**Branch:** `fix/migration-race-flock`

**Approach:**
- Uses `github.com/gofrs/flock` for cross-platform file locking
- Creates a lock file (`.mcp-toolkit-migration.lock`) in the same directory as the database
- Only one process can hold the lock at a time
- Other processes wait up to 30 seconds for the lock
- Works on Linux, macOS, Windows

**Changes:**
- Added dependency: `github.com/gofrs/flock v0.13.0`
- Modified `pkg/db/db.go` to acquire file lock before migrations

**Pros:**
- Most robust - prevents any concurrent migrations
- Works across all processes system-wide
- Battle-tested library
- Clean failure mode with timeout

**Cons:**
- Adds external dependency (~500 LOC)
- Slightly more complex

**File:** `/Users/bobby/git/mcp-gateway-solution-a/pkg/db/db.go:100-122`

---

### Solution B: Enhanced SQLite Retry Logic
**Location:** `../mcp-gateway-solution-b`
**Branch:** `fix/migration-race-sqlite-retry`

**Approach:**
- Custom retry loop that specifically handles `ErrDirty`
- 20 retries with exponential backoff (200ms → 300ms → 450ms → ...)
- Detects when another process is migrating and waits
- No external dependencies

**Changes:**
- Modified `pkg/db/db.go` to add smart retry logic
- Removed usage of `pkg/retry` package in favor of custom loop

**Pros:**
- No external dependencies
- Smart backoff strategy
- Clear error messages
- Lightweight

**Cons:**
- Processes still attempt to migrate (just retry on failure)
- Longer total wait time in worst case
- More complex retry logic

**File:** `/Users/bobby/git/mcp-gateway-solution-b/pkg/db/db.go:96-135`

---

### Solution C: Retry on ErrDirty using existing retry package
**Location:** `../mcp-gateway-solution-c`
**Branch:** `fix/migration-race-errdirty-retry`

**Approach:**
- Uses existing `pkg/retry` package with custom predicate
- Retries only on `ErrDirty` errors (not all errors)
- 15 attempts with 400ms fixed delay
- Minimal code changes

**Changes:**
- Modified `pkg/db/db.go` to use `retry.If()` with ErrDirty predicate
- Simple and leverages existing infrastructure

**Pros:**
- Minimal code changes
- Uses existing retry infrastructure
- No external dependencies
- Easy to understand

**Cons:**
- Fixed delay (no exponential backoff)
- Processes still attempt to migrate
- Less sophisticated than Solution B

**File:** `/Users/bobby/git/mcp-gateway-solution-c/pkg/db/db.go:96-120`

## Testing Each Solution

### 1. Copy the test program to each worktree:

```bash
cp test_migration_race.go ../mcp-gateway-solution-a/
cp test_migration_race.go ../mcp-gateway-solution-b/
cp test_migration_race.go ../mcp-gateway-solution-c/
```

### 2. Test Solution A (File Locking):

```bash
cd ../mcp-gateway-solution-a
make docker-mcp  # Installs to ~/.docker/cli-plugins/
go run test_migration_race.go
```

### 3. Test Solution B (Enhanced SQLite Retry):

```bash
cd ../mcp-gateway-solution-b
make docker-mcp  # Installs to ~/.docker/cli-plugins/
go run test_migration_race.go
```

### 4. Test Solution C (ErrDirty Retry):

```bash
cd ../mcp-gateway-solution-c
make docker-mcp  # Installs to ~/.docker/cli-plugins/
go run test_migration_race.go
```

## Recommendation

**Solution A (File Locking)** is recommended because:

1. ✅ **Prevents the race condition entirely** - only one process can migrate at a time
2. ✅ **Works system-wide** - all processes respect the lock
3. ✅ **Clean failure mode** - timeout with clear error message
4. ✅ **Cross-platform** - works on Linux, macOS, Windows
5. ✅ **Battle-tested** - `gofrs/flock` is used by Docker Compose, Kubernetes tools, etc.
6. ✅ **Small dependency** - minimal overhead

Solutions B and C are viable alternatives if avoiding dependencies is critical, but they allow all processes to attempt migration and only handle failures after the fact.

## Next Steps

1. Test each solution with `test_migration_race.go`
2. Compare success rates and performance
3. Choose the best solution
4. Create a PR from the chosen branch
5. Clean up unused worktrees

## Worktree Management

View all worktrees:
```bash
git worktree list
```

Remove a worktree when done:
```bash
git worktree remove ../mcp-gateway-solution-a
git worktree remove ../mcp-gateway-solution-b
git worktree remove ../mcp-gateway-solution-c
```

Delete the branches if not merging:
```bash
git branch -D fix/migration-race-flock
git branch -D fix/migration-race-sqlite-retry
git branch -D fix/migration-race-errdirty-retry
```
