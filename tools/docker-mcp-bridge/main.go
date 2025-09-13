package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	if err := run(ctx); err != nil {
		panic(err)
	}
}

func run(ctx context.Context) error {
	ln, err := net.Listen("tcp", ":4444")
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			ln.Close()
			return ctx.Err()
		default:
			conn, err := ln.Accept()
			if err != nil {
				log.Println("Error accepting connection:", err)
				continue
			}

			go func() {
				cmd := exec.CommandContext(ctx, os.Args[1], os.Args[2:]...)
				cmd.Stdin = conn
				cmd.Stdout = conn
				if err := cmd.Run(); err != nil {
					log.Println("Error running command:", err)
				}
			}()
		}
	}
}
