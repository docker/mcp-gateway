package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	dbPath     = "/Users/bobby/.docker/mcp/mcp-toolkit.db"
	iterations = 100
)

type result struct {
	iteration int
	success   bool
	output    string
	err       error
	duration  time.Duration
}

func main() {
	fmt.Printf("Starting migration race test with %d concurrent iterations...\n", iterations)

	// Delete the database once at the beginning
	fmt.Printf("Deleting database at %s...\n", dbPath)
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove db: %v\n", err)
	} else {
		fmt.Println("Database deleted successfully")
	}

	fmt.Println("\nLaunching all processes simultaneously...")

	var wg sync.WaitGroup
	results := make(chan result, iterations)

	startTime := time.Now()

	// Launch all iterations concurrently - they will all race to initialize the database
	for i := range iterations {
		wg.Add(1)
		go func(iterNum int) {
			defer wg.Done()

			iterStart := time.Now()

			// Run docker mcp profile list - all will try to initialize/migrate at once
			ctx := context.Background()
			cmd := exec.CommandContext(ctx, "docker", "mcp", "profile", "list")
			output, err := cmd.CombinedOutput()

			results <- result{
				iteration: iterNum,
				success:   err == nil,
				output:    string(output),
				err:       err,
				duration:  time.Since(iterStart),
			}
		}(i)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and analyze results
	var (
		successCount    int
		failureCount    int
		migrationErrors int
		dirtyDBErrors   int
	)

	fmt.Println("Results:")
	fmt.Println("--------")

	for res := range results {
		if res.success {
			successCount++
			fmt.Printf("[%3d] ✓ Success (took %v)\n", res.iteration, res.duration)
		} else {
			failureCount++
			fmt.Printf("[%3d] ✗ FAILED (took %v)\n", res.iteration, res.duration)
			fmt.Printf("      Error: %v\n", res.err)

			if res.output != "" {
				fmt.Printf("      Output: %s\n", res.output)
			}

			// Check for specific error patterns
			outputStr := res.output
			if res.err != nil {
				outputStr += res.err.Error()
			}

			if containsStr(outputStr, "failed to run migrations") {
				migrationErrors++
			}
			if containsStr(outputStr, "Dirty database version") {
				dirtyDBErrors++
			}
		}
	}

	totalDuration := time.Since(startTime)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Summary:")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total iterations:      %d\n", iterations)
	fmt.Printf("Successful:            %d (%.1f%%)\n", successCount, float64(successCount)/float64(iterations)*100)
	fmt.Printf("Failed:                %d (%.1f%%)\n", failureCount, float64(failureCount)/float64(iterations)*100)
	fmt.Printf("Migration errors:      %d\n", migrationErrors)
	fmt.Printf("Dirty DB errors:       %d\n", dirtyDBErrors)
	fmt.Printf("Total time:            %v\n", totalDuration)
	fmt.Printf("Avg time per iter:     %v\n", totalDuration/time.Duration(iterations))

	if dirtyDBErrors > 0 {
		fmt.Println("\n🎯 SUCCESS! Reproduced the 'Dirty database version' error!")
	} else if migrationErrors > 0 {
		fmt.Println("\n⚠️  Found migration errors but not the specific 'Dirty database' error")
	} else if failureCount > 0 {
		fmt.Println("\n⚠️  Found failures but not migration-related")
	} else {
		fmt.Println("\n✓ All iterations succeeded (could not reproduce the error)")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
