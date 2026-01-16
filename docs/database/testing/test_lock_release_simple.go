package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

// This is a simple program that demonstrates lock behavior.
// Run it manually to see the behavior:
//
// Terminal 1: go run test_lock_release_simple.go acquire
// Terminal 2: go run test_lock_release_simple.go check
// Terminal 1: (kill with Ctrl+C or kill -9)
// Terminal 2: go run test_lock_release_simple.go check    (now succeeds!)
// Terminal 2: go run test_lock_release_simple.go cleanup

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	lockFile := filepath.Join(os.TempDir(), "flock-test.lock")
	fmt.Println(lockFile)

	switch os.Args[1] {
	case "acquire":
		acquireLock(lockFile)
	case "check":
		checkLock(lockFile)
	case "status":
		showStatus(lockFile)
	case "cleanup":
		cleanup(lockFile)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run test_lock_release_simple.go acquire   - Acquire lock and hold it")
	fmt.Println("  go run test_lock_release_simple.go check     - Try to acquire lock")
	fmt.Println("  go run test_lock_release_simple.go status    - Check if lockfile exists")
	fmt.Println("  go run test_lock_release_simple.go cleanup   - Remove lockfile")
	fmt.Println()
	fmt.Println("Test procedure:")
	fmt.Println("  1. Terminal 1: run 'acquire' - holds lock indefinitely")
	fmt.Println("  2. Terminal 2: run 'check' - should fail (lock held)")
	fmt.Println("  3. Terminal 1: kill with Ctrl+C or kill -9")
	fmt.Println("  4. Terminal 2: run 'check' again - should succeed!")
	fmt.Println("  5. Terminal 2: run 'status' - lockfile still exists")
	fmt.Println("  6. Terminal 2: run 'cleanup' - remove lockfile")
}

func acquireLock(lockFile string) {
	fileLock := flock.New(lockFile)

	fmt.Printf("[PID %d] Attempting to acquire lock on: %s\n", os.Getpid(), lockFile)

	locked, err := fileLock.TryLock()
	if err != nil {
		fmt.Printf("❌ Error acquiring lock: %v\n", err)
		os.Exit(1)
	}

	if !locked {
		fmt.Printf("❌ Could not acquire lock (already held by another process)\n")
		os.Exit(1)
	}

	fmt.Printf("✅ Lock acquired successfully!\n")
	fmt.Printf("   Lockfile: %s\n", lockFile)
	fmt.Printf("   PID: %d\n", os.Getpid())
	fmt.Println()
	fmt.Println("Holding lock indefinitely. Press Ctrl+C to exit gracefully,")
	fmt.Println("or use 'kill -9' from another terminal to simulate a crash.")
	fmt.Println()
	fmt.Println("Commands to try in another terminal:")
	fmt.Printf("  go run test_lock_release_simple.go check    # Try to acquire (should fail)\n")
	fmt.Printf("  kill -9 %d                                  # Kill this process\n", os.Getpid())
	fmt.Printf("  go run test_lock_release_simple.go check    # Try again (should succeed!)\n")
	fmt.Println()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal, releasing lock...")
		fileLock.Unlock()
		fmt.Println("Lock released. Exiting.")
		os.Exit(0)
	}()

	// Hold lock forever
	for {
		fmt.Printf(".")
		time.Sleep(2 * time.Second)
	}
}

func checkLock(lockFile string) {
	fileLock := flock.New(lockFile)

	fmt.Printf("[PID %d] Attempting to acquire lock on: %s\n", os.Getpid(), lockFile)

	locked, err := fileLock.TryLock()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if !locked {
		fmt.Printf("❌ Lock is currently held by another process\n")
		fmt.Printf("   This means another process has the lock\n")
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully acquired lock!\n")
	fmt.Printf("   This means no other process holds the lock\n")
	fmt.Printf("   (The lock was either never acquired, or the holding process died)\n")

	// Release it immediately
	fileLock.Unlock()
	fmt.Println("\nLock released.")
}

func showStatus(lockFile string) {
	fmt.Printf("Lockfile path: %s\n", lockFile)

	info, err := os.Stat(lockFile)
	if os.IsNotExist(err) {
		fmt.Println("Status: ❌ Lockfile does not exist")
		return
	}

	if err != nil {
		fmt.Printf("Error checking file: %v\n", err)
		return
	}

	fmt.Printf("Status: ✅ Lockfile exists\n")
	fmt.Printf("  Size: %d bytes\n", info.Size())
	fmt.Printf("  Modified: %s\n", info.ModTime().Format(time.RFC3339))
	fmt.Println("\nNote: The lockfile existing does NOT mean the lock is held.")
	fmt.Println("      Run 'check' to see if the lock can be acquired.")
}

func cleanup(lockFile string) {
	fmt.Printf("Removing lockfile: %s\n", lockFile)

	err := os.Remove(lockFile)
	if os.IsNotExist(err) {
		fmt.Println("✅ Lockfile doesn't exist (already clean)")
		return
	}

	if err != nil {
		fmt.Printf("❌ Error removing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Lockfile removed successfully")
}
