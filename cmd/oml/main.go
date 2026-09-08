package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

func main() {
	var (
		flagPort    = flag.String("port", "", "serial port (e.g. /dev/ttyACM0 or COM3)")
		flagBaud    = flag.Int("baud", 115200, "baud rate")
		flagProfile = flag.String("profile", "", "profile name or path (.yaml)")
		flagFile    = flag.String("file", "", "replay a saved log file instead of a serial port")
		flagVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Printf("oml (oh-my-logs) %s\n", version)
		os.Exit(0)
	}

	if *flagPort != "" && *flagFile != "" {
		fmt.Fprintln(os.Stderr, "error: --port and --file are mutually exclusive")
		os.Exit(1)
	}

	// ── App config & profile discovery ───────────────────────────────────────
	appCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config: %v\n", err)
	}

	// ── Load profile ─────────────────────────────────────────────────────────
	var profile *parser.Profile
	var p parser.Parser

	profilePath := resolveProfilePath(appCfg, *flagProfile)
	if profilePath != "" {
		profile, err = parser.LoadProfile(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		p, err = profile.BuildParser()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		p = parser.NewRawParser()
	}

	// ── Serial / file source ─────────────────────────────────────────────────
	var src serial.Source
	serialCfg := serial.DefaultConfig()
	serialCfg.Baud = *flagBaud

	if *flagFile != "" {
		src, err = serial.NewFileSource(*flagFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else if *flagPort != "" {
		serialCfg.Port = *flagPort
		src, err = serial.NewSerialSource(serialCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	// If neither port nor file was given, we start with no source connected.

	// ── Ring buffer ───────────────────────────────────────────────────────────
	buf := record.NewBuffer(record.DefaultCapacity)

	// ── TUI ───────────────────────────────────────────────────────────────────
	model := tui.New(serialCfg, profile, p, buf, src)

	prog := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// resolveProfilePath looks up a profile by name in the config profiles dir,
// or treats the flag value as a direct path if it ends in .yaml/.yml.
func resolveProfilePath(appCfg *config.AppConfig, nameOrPath string) string {
	if nameOrPath == "" {
		return ""
	}
	// Direct path.
	if _, err := os.Stat(nameOrPath); err == nil {
		return nameOrPath
	}
	if appCfg == nil {
		return ""
	}
	// Search config profiles dir.
	files, _ := appCfg.ListProfileFiles()
	for _, f := range files {
		prof, err := parser.LoadProfile(f)
		if err != nil {
			continue
		}
		if prof.Name == nameOrPath {
			return f
		}
	}
	return ""
}
