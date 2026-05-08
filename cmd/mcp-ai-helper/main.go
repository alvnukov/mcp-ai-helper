// Package main starts the mcp-ai-helper stdio MCP server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/config"
	mcpserver "github.com/zol/mcp-ai-helper/internal/mcp"
)

func main() {
	configPath := flag.String("config", config.DefaultConfigPath(), "path to config yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	srv := mcpserver.New(cfg)
	if err := server.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "serve stdio: %v\n", err)
		os.Exit(1)
	}
}
