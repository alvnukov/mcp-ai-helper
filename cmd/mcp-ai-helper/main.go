// Package main starts the mcp-ai-helper stdio MCP server, and registers it with
// AI clients or removes it again.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
	mcpserver "github.com/alvnukov/mcp-ai-helper/internal/mcp"
	"github.com/alvnukov/mcp-ai-helper/internal/setup"
)

// version is set from an immutable release tag through -ldflags.
var version = "dev"

func versionLine() string {
	return "mcp-ai-helper " + version
}

func main() {
	// With no subcommand the binary is the server, which is what an MCP client
	// launches and therefore what must stay the default.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "version":
			fmt.Println(versionLine())
			return
		case "setup":
			runSetup(os.Args[1], os.Args[2:], setup.Run)
			return
		case "remove", "uninstall":
			runSetup(os.Args[1], os.Args[2:], setup.Remove)
			return
		}
	}
	serve()
}

func serve() {
	configPath := flag.String("config", config.DefaultConfigPath(), "path to config yaml")
	flag.Usage = usage
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	srv := mcpserver.New(cfg)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		signal.Stop(sig)
		fmt.Fprintf(os.Stderr, "mcp-ai-helper: received signal, shutting down\n")
		os.Exit(0)
	}()

	if err := server.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "serve stdio: %v\n", err)
		os.Exit(1)
	}
}

func runSetup(name string, argv []string, run func(setup.Options, io.Writer) error) {
	opts, err := parseSetupArgs(name, argv)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		}
		os.Exit(2)
	}
	if err := run(opts, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}

func parseSetupArgs(name string, argv []string) (setup.Options, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	verb, past := "configure", "registered in"
	if name != "setup" {
		verb, past = "clean up", "removed from"
	}

	var requested clientList
	fs.Var(&requested, "c", "clients to "+verb+", comma-separated: claude,codex,opencode")
	fs.Var(&requested, "clients", "alias for -c")
	global := fs.Bool("global", false, "act on the user-wide config instead of the project one")
	dryRun := fs.Bool("dry-run", false, "report what would change instead of writing it")
	noInstructions := fs.Bool("no-instructions", false, "leave the mcp-ai-helper block in CLAUDE.md / AGENTS.md alone")
	noSkills := fs.Bool("no-skills", false, "leave the mcp-ai-helper skills alone")
	configPath := fs.String("config", "", "pin --config PATH in the command line the client runs; omit to let the server use its own default")
	fs.Usage = func() {
		if _, err := fmt.Fprintf(fs.Output(), "Usage: mcp-ai-helper %s -c claude,codex,opencode [flags]\n\n", name); err != nil {
			return
		}
		if _, err := fmt.Fprintf(fs.Output(), "The MCP server entry, the instructions block and the skills are %s each client.\n\n", past); err != nil {
			return
		}
		fs.PrintDefaults()
	}

	if err := fs.Parse(argv); err != nil {
		return setup.Options{}, err
	}
	if len(requested) == 0 {
		fs.Usage()
		return setup.Options{}, errors.New("no clients given")
	}

	return setup.Options{
		Clients:        requested,
		Global:         *global,
		DryRun:         *dryRun,
		NoInstructions: *noInstructions,
		NoSkills:       *noSkills,
		ConfigPath:     *configPath,
	}, nil
}

// clientList collects a repeatable, comma-separated flag, so that both
// `-c claude,codex` and `-c claude -c codex` mean the same thing.
type clientList []string

func (l *clientList) String() string { return strings.Join(*l, ",") }

func (l *clientList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			*l = append(*l, item)
		}
	}
	return nil
}

func usage() {
	out := flag.CommandLine.Output()
	if _, err := fmt.Fprint(out, `Usage: mcp-ai-helper [flags]              start the stdio MCP server
       mcp-ai-helper setup -c CLIENTS     register the server in your AI clients
       mcp-ai-helper remove -c CLIENTS    remove it again (alias: uninstall)

Clients: claude, codex, opencode. Run a subcommand with -h for its flags.

Server flags:
`); err != nil {
		return
	}
	flag.PrintDefaults()
}
