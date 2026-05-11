// Package main starts the local task browser UI for the current agent session.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
	mcpserver "github.com/zol/mcp-ai-helper/internal/mcp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18067", "local HTTP address for the task UI")
	repoPath := flag.String("repo", ".", "repository path to prefill in the task UI")
	configPath := flag.String("config", config.DefaultConfigPath(), "path to config yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	prefillRepo := ""
	if *repoPath != "" {
		prefillRepo, err = filepath.Abs(*repoPath)
		if err != nil {
			log.Fatalf("resolve repo: %v", err)
		}
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}

	uiURL := url.URL{Scheme: "http", Host: listener.Addr().String(), Path: "/"}
	if prefillRepo != "" {
		query := uiURL.Query()
		query.Set("repo_path", prefillRepo)
		uiURL.RawQuery = query.Encode()
	}
	fmt.Fprintln(os.Stdout, uiURL.String())
	log.Printf("task UI listening on %s", uiURL.String())

	server := &http.Server{
		Handler:           mcpserver.NewTaskUIHandler(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		signal.Stop(sigCh)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve task UI: %v", err)
	}
}
