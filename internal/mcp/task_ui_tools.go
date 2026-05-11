package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const defaultTaskUIAddr = "127.0.0.1:18067"

type taskUIStartRequest struct {
	RepoPath string `json:"repo_path"`
	Addr     string `json:"addr"`
}

type taskUIStartResult struct {
	URL            string `json:"url"`
	BaseURL        string `json:"base_url"`
	RepoPath       string `json:"repo_path"`
	Addr           string `json:"addr"`
	AlreadyRunning bool   `json:"already_running"`
}

type taskUIStopResult struct {
	Stopped bool `json:"stopped"`
}

type taskUIServer struct {
	server  *http.Server
	baseURL string
	addr    string
}

func registerTaskUITools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("task_ui_start",
		basemcp.WithDescription("Start the local task browser HTTP UI inside the current MCP helper process and return a browser URL."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository path to prefill in the UI.")),
		basemcp.WithString("addr", basemcp.Description("Optional loopback TCP address. Defaults to 127.0.0.1:18067.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args taskUIStartRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := deps.startTaskUI(ctx, args.RepoPath, args.Addr)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("task_ui_stop",
		basemcp.WithDescription("Stop the in-process local task browser HTTP UI if it is running."),
	), func(ctx context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		stopped, err := deps.stopTaskUI(ctx)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(taskUIStopResult{Stopped: stopped})
	})
}

func (s *Server) startTaskUI(_ context.Context, repoPath string, addr string) (taskUIStartResult, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return taskUIStartResult{}, errors.New("repo_path is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskUI != nil {
		return taskUIStartResult{URL: taskUIURL(s.taskUI.baseURL, repoPath), BaseURL: s.taskUI.baseURL, RepoPath: repoPath, Addr: s.taskUI.addr, AlreadyRunning: true}, nil
	}

	listener, baseURL, listenAddr, err := listenTaskUI(addr)
	if err != nil {
		return taskUIStartResult{}, err
	}
	ui := &taskUIServer{
		server:  &http.Server{Handler: newServerTaskUIHandler(s), ReadHeaderTimeout: 5 * time.Second},
		baseURL: baseURL,
		addr:    listenAddr,
	}
	s.taskUI = ui
	go func() {
		if err := ui.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "mcp-ai-helper: task UI server stopped: %v\n", err)
		}
		s.mu.Lock()
		if s.taskUI == ui {
			s.taskUI = nil
		}
		s.mu.Unlock()
	}()

	return taskUIStartResult{URL: taskUIURL(baseURL, repoPath), BaseURL: baseURL, RepoPath: repoPath, Addr: listenAddr}, nil
}

func (s *Server) stopTaskUI(ctx context.Context) (bool, error) {
	s.mu.Lock()
	ui := s.taskUI
	s.taskUI = nil
	s.mu.Unlock()
	if ui == nil {
		return false, nil
	}
	return true, ui.server.Shutdown(ctx)
}

func listenTaskUI(addr string) (net.Listener, string, string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultTaskUIAddr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.Count(addr, ":") == 0 {
			host = "127.0.0.1"
			port = addr
		} else {
			return nil, "", "", fmt.Errorf("invalid task UI addr %q: %w", addr, err)
		}
	}
	if strings.TrimSpace(port) == "" {
		return nil, "", "", errors.New("task UI addr port is required")
	}
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	if host != "localhost" {
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !ip.IsLoopback() {
			return nil, "", "", fmt.Errorf("task UI addr must bind to loopback, got %q", host)
		}
	}
	listenAddr := net.JoinHostPort(host, port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, "", "", err
	}
	displayPort := port
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		displayPort = strconv.Itoa(tcpAddr.Port)
	}
	base := url.URL{Scheme: "http", Host: net.JoinHostPort(host, displayPort), Path: "/"}
	return listener, base.String(), net.JoinHostPort(host, displayPort), nil
}

func taskUIURL(baseURL string, repoPath string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	query := parsed.Query()
	if strings.TrimSpace(repoPath) != "" {
		query.Set("repo_path", repoPath)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
