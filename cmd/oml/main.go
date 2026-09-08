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

const (
	version  = "1.0.0"
	codename = "Joseense"
)

func main() {
	var (
		flagPort          = flag.String("port", "", "serial port (e.g. /dev/ttyACM0 or COM3)")
		flagBaud          = flag.Int("baud", 115200, "baud rate")
		flagProfile       = flag.String("profile", "", "profile name or path (.yaml)")
		flagFile          = flag.String("file", "", "replay a saved log file instead of a serial port")
		flagVersion       = flag.Bool("version", false, "print version and exit")
		flagImportProfile = flag.String("import-profile", "", "import a YAML profile into the user profiles directory")
		flagExportProfile = flag.String("export-profile", "", "export a profile by name (prints YAML to stdout or saves to --out)")
		flagListProfiles  = flag.Bool("list-profiles", false, "list all available profiles and their locations")
		flagProfilesDir   = flag.Bool("profiles-dir", false, "print the global OS profiles directory path")
		flagOut           = flag.String("out", "", "output file path for --export-profile")
		flagInstall       = flag.Bool("install", false, "install oml binary into system/user PATH")
		flagUninstall     = flag.Bool("uninstall", false, "uninstall oml binary from system/user PATH")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Printf("oml (oh-my-logs) v%s - %s\n", version, codename)
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

	if *flagInstall {
		dest, err := config.InstallBinary(appCfg, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error installing binary: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Successfully installed oml to:\n  %s\n", dest)
		fmt.Println("\nYou can now run 'oml' from anywhere in your terminal.")
		os.Exit(0)
	}

	if *flagUninstall {
		dest, err := config.UninstallBinary("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error uninstalling binary: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Successfully uninstalled oml from:\n  %s\n", dest)
		os.Exit(0)
	}

	if *flagProfilesDir {
		if appCfg != nil {
			fmt.Println(appCfg.ProfilesDir)
		}
		os.Exit(0)
	}

	if *flagListProfiles {
		list := config.ListProfiles(appCfg)
		if len(list) == 0 {
			fmt.Println("No profiles found.")
			if appCfg != nil {
				fmt.Printf("Global profiles directory: %s\n", appCfg.ProfilesDir)
			}
			os.Exit(0)
		}
		fmt.Printf("Profiles (%d available):\n", len(list))
		for _, p := range list {
			loc := "local"
			if p.IsGlobal {
				loc = "global"
			}
			fmt.Printf("  • %-12s [%-6s]  %-8s  %s\n", p.Name, loc, p.ParserType, p.Path)
		}
		if appCfg != nil {
			fmt.Printf("\nGlobal profiles directory: %s\n", appCfg.ProfilesDir)
		}
		os.Exit(0)
	}

	if *flagImportProfile != "" {
		target, name, err := config.ImportProfile(appCfg, *flagImportProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error importing profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Successfully imported profile %q to:\n  %s\n", name, target)
		fmt.Printf("\nYou can now run: oml --profile %s\n", name)
		os.Exit(0)
	}

	if *flagExportProfile != "" {
		content, sourcePath, err := config.ExportProfile(appCfg, *flagExportProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error exporting profile: %v\n", err)
			os.Exit(1)
		}
		if *flagOut != "" {
			if err := os.WriteFile(*flagOut, content, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing to %q: %v\n", *flagOut, err)
				os.Exit(1)
			}
			fmt.Printf("✓ Exported profile %q from %s to %s\n", *flagExportProfile, sourcePath, *flagOut)
		} else {
			os.Stdout.Write(content)
		}
		os.Exit(0)
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

