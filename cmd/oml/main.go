package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	version  = "1.2.0"
	codename = "Taubateano"
)

func main() {
	var (
		flagPort          = flag.String("port", "", "serial port (e.g. /dev/ttyACM0 or COM3)")
		flagBaud          = flag.Int("baud", 115200, "baud rate")
		flagProfile       = flag.String("profile", "", "profile name or path (.yaml)")
		flagFile          = flag.String("file", "", "replay a saved log file instead of a serial port")
		flagCmd           = flag.String("cmd", "", "run external shell command as live log stream (alias: --exec)")
		flagExec          = flag.String("exec", "", "run external shell command as live log stream (alias: --cmd)")
		flagStdin         = flag.Bool("stdin", false, "read log stream from standard input")
		flagVersion       = flag.Bool("version", false, "print version and exit")
		flagImportProfile = flag.String("import-profile", "", "import a YAML profile into the user profiles directory")
		flagExportProfile = flag.String("export-profile", "", "export a profile by name (prints YAML to stdout or saves to --out)")
		flagListProfiles  = flag.Bool("list-profiles", false, "list all available profiles and their locations")
		flagProfilesDir   = flag.Bool("profiles-dir", false, "print the global OS profiles directory path")
		flagOut           = flag.String("out", "", "output file path for --export-profile")
		flagInstall       = flag.Bool("install", false, "install oml binary into system/user PATH")
		flagUninstall     = flag.Bool("uninstall", false, "uninstall oml binary from system/user PATH")
		flagUpdate        = flag.Bool("update", false, "check for and install latest oml release")
		flagNightly       = flag.Bool("nightly", false, "use nightly build channel")
		flagTee           = flag.String("tee", "", "stream raw incoming lines directly to specified log file")
		flagDirectToDisk  = flag.Bool("direct-to-disk", false, "enable direct-to-disk continuous logging")
		flagPrefix        = flag.String("prefix", "", "filename prefix for direct-to-disk logs (default: oml)")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Printf("oml (oh-my-logs) v%s - %s\n", version, codename)
		os.Exit(0)
	}

	if *flagCmd == "" && *flagExec != "" {
		*flagCmd = *flagExec
	}

	sourceCount := 0
	if *flagPort != "" {
		sourceCount++
	}
	if *flagFile != "" {
		sourceCount++
	}
	if *flagCmd != "" {
		sourceCount++
	}
	if *flagStdin {
		sourceCount++
	}
	if sourceCount > 1 {
		fmt.Fprintln(os.Stderr, "error: --port, --file, --cmd, and --stdin are mutually exclusive")
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

	if *flagUpdate {
		channel := "latest stable release"
		if *flagNightly {
			channel = "nightly build"
		}
		fmt.Printf("Checking for updates (%s, current: v%s)...\n", channel, version)
		msg, err := config.UpdateBinary(appCfg, version, *flagNightly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error updating oml: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(msg)
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

	var usingStdin bool

	// Auto-detect piped standard input if no other source flag is explicitly provided
	if sourceCount == 0 && isStdinPiped() {
		*flagStdin = true
	}

	if *flagFile != "" {
		src, err = serial.NewFileSource(*flagFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else if *flagCmd != "" {
		src, err = serial.NewProcessSource(*flagCmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else if *flagStdin {
		src = serial.NewPipeSource(os.Stdin)
		usingStdin = true
	} else if *flagPort != "" {
		serialCfg.Port = *flagPort
		src, err = serial.NewSerialSource(serialCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else if savedSettings != nil && savedSettings.Port != "" {
		serialCfg.Port = savedSettings.Port
		// Keep last used port pre-configured in settings, but start in clean
		// disconnected state so launch does not lock busy ports or loop.
		// User can press 'r' to connect when ready.
	}

	// ── Ring buffer ───────────────────────────────────────────────────────────
	bufCap := record.DefaultCapacity
	if savedSettings != nil && savedSettings.BufferCapacity > 0 {
		bufCap = savedSettings.BufferCapacity
	}
	buf := record.NewBuffer(bufCap)

	// ── TUI ───────────────────────────────────────────────────────────────────
	if savedSettings != nil && savedSettings.Theme != "" {
		tui.SetCurrentTheme(savedSettings.Theme)
	}
	model := tui.New(serialCfg, profile, p, buf, src, appCfg)

	if *flagPrefix != "" {
		model.SetDirectToDiskPrefix(*flagPrefix)
	}

	if *flagTee != "" {
		if err := model.StartDiskLogger(*flagTee); err != nil {
			fmt.Fprintf(os.Stderr, "warning: direct-to-disk: %v\n", err)
		}
	} else if *flagDirectToDisk {
		if err := model.StartDiskLogger(""); err != nil {
			fmt.Fprintf(os.Stderr, "warning: direct-to-disk: %v\n", err)
		}
	}

	var teaOpts []tea.ProgramOption
	teaOpts = append(teaOpts, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if usingStdin {
		tty, err := openControllingTTY()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot open controlling terminal: %v\n", err)
			os.Exit(1)
		}
		defer tty.Close()
		teaOpts = append(teaOpts, tea.WithInput(tty))
	}

	prog := tea.NewProgram(
		model,
		teaOpts...,
	)

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func isStdinPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func openControllingTTY() (*os.File, error) {
	if runtime.GOOS == "windows" {
		return os.OpenFile("CONIN$", os.O_RDWR, 0)
	}
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

