package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/mcp"
)

func runMCPCommand(appCfg *config.AppConfig, args []string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	port := fs.String("port", "", "serial port to auto-connect on startup (optional)")
	baud := fs.Int("baud", 115200, "serial baud rate")
	profile := fs.String("profile", "", "profile name or YAML path to activate")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: oml mcp [options]

Start headless Model Context Protocol (MCP) server over standard I/O (JSON-RPC 2.0).
Enables AI coding assistants (Gemini CLI, Claude Desktop, Antigravity, Cursor)
to inspect serial logs, execute shell commands, and control port connections.

Options:
  --port <device>     Initial serial port to connect (optional)
  --baud <rate>       Serial baud rate (default: 115200)
  --profile <name>    Initial parsing profile (default: raw)
  -h, --help          Show this help message
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	session := mcp.NewSession(appCfg, 50_000)

	// If initial port was specified, attempt connection immediately
	if *port != "" {
		if err := session.Connect(*port, *baud, *profile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: initial connection to %s failed: %v\n", *port, err)
		} else {
			fmt.Fprintf(os.Stderr, "connected to %s at %d baud\n", *port, *baud)
		}
	}

	server := mcp.NewServer(session, appCfg, version)
	if err := server.ServeStdio(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		return 1
	}

	return 0
}
