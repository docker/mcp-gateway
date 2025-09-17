package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func ReadUserInput(prompt string) (string, error) {
	fmt.Print(prompt)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	defer signal.Stop(sigChan)
	inputChan := make(chan string)
	errChan := make(chan error)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			errChan <- err
			return
		}
		inputChan <- input
	}()
	select {
	case <-sigChan:
		return "", nil
	case err := <-errChan:
		return "", err
	case input := <-inputChan:
		return strings.TrimSpace(input), nil
	}
}

func ReadAllWithContext(ctx context.Context, r io.Reader) ([]byte, error) {
	lines := make(chan []byte)
	errs := make(chan error)

	go func() {
		defer close(lines)
		defer close(errs)

		reader := bufio.NewReader(r)
		line, err := reader.ReadBytes('\n')
		switch {
		case err == io.EOF:
			lines <- line
		case err != nil:
			errs <- err
		default:
			lines <- line
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errs:
		return nil, err
	case line := <-lines:
		return line, nil
	}
}
