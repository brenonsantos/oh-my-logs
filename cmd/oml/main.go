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

	// ── App config & persistent settings ─────────────────────────────────────
	appCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config: %v\n", err)
	}

	savedSettings, _ := appCfg.LoadSettings()

	// ── Resolve profile ───────────────────────────────────────────────────────
	targetProfile := *flagProfile
	if targetProfile == "" && savedSettings != nil && savedSettings.Profile != "" {
		targetProfile = savedSettings.Profile
	}

	var profile *parser.Profile
	var p parser.Parser

	profilePath := config.ResolveProfilePath(appCfg, targetProfile)
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

	// Determine if -baud was explicitly provided on CLI
	baudProvided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "baud" {
			baudProvided = true
		}
	})
	if baudProvided {
		serialCfg.Baud = *flagBaud
	} else if savedSettings != nil && savedSettings.Baud > 0 {
		serialCfg.Baud = savedSettings.Baud
	} else {
		serialCfg.Baud = *flagBaud
	}

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
	} else if savedSettings != nil && savedSettings.Port != "" {
		serialCfg.Port = savedSettings.Port
		// Auto-connect to last used port if device is plugged in.
		// If disconnected, src remains nil and TUI displays disconnected empty state.
		src, _ = serial.NewSerialSource(serialCfg)
	}

	// ── Ring buffer ───────────────────────────────────────────────────────────
	buf := record.NewBuffer(record.DefaultCapacity)

	// ── TUI ───────────────────────────────────────────────────────────────────
	model := tui.New(serialCfg, profile, p, buf, src, appCfg)

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

