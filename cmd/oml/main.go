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
	version  = "1.3.0"
	codename = "Araquariense"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "profile" || os.Args[1] == "profiles") {
		appCfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: config: %v\n", err)
		}
		os.Exit(runProfileCommand(appCfg, os.Args[2:]))
	}

	var (
		flagPort             = flag.String("port", "", "serial port (e.g. /dev/ttyACM0 or COM3)")
		flagBaud             = flag.Int("baud", 115200, "baud rate")
		flagProfile          = flag.String("profile", "", "profile name or path (.yaml)")
		flagFile             = flag.String("file", "", "replay a saved log file instead of a serial port")
		flagCmd              = flag.String("cmd", "", "run external shell command as live log stream (alias: --exec)")
		flagExec             = flag.String("exec", "", "run external shell command as live log stream (alias: --cmd)")
		flagStdin            = flag.Bool("stdin", false, "read log stream from standard input")
		flagVersion          = flag.Bool("version", false, "print version and exit")
		flagImportProfile    = flag.String("import-profile", "", "import a YAML profile into the user profiles directory")
		flagInstallProfile   = flag.String("install-profile", "", "install profile from file, directory, or URL (alias: --import-profile)")
		flagUninstallProfile = flag.String("uninstall-profile", "", "uninstall profile by ID (#1) or name")
		flagRemoveProfile    = flag.String("remove-profile", "", "uninstall profile by ID (#1) or name (alias: --uninstall-profile)")
		flagExportProfile    = flag.String("export-profile", "", "export a profile by name (prints YAML to stdout or saves to --out)")
		flagListProfiles     = flag.Bool("list-profiles", false, "list all available profiles and their locations")
		flagProfilesDir      = flag.Bool("profiles-dir", false, "print the global OS profiles directory path")
		flagOut              = flag.String("out", "", "output file path for --export-profile")
		flagInstall          = flag.Bool("install", false, "install oml binary into system/user PATH")
		flagUninstall        = flag.Bool("uninstall", false, "uninstall oml binary from system/user PATH")
		flagUpdate           = flag.Bool("update", false, "check for and install latest oml release")
		flagNightly          = flag.Bool("nightly", false, "use nightly build channel")
		flagTee              = flag.String("tee", "", "stream raw incoming lines directly to specified log file")
		flagDirectToDisk     = flag.Bool("direct-to-disk", false, "enable direct-to-disk continuous logging")
		flagPrefix           = flag.String("prefix", "", "filename prefix for direct-to-disk logs (default: oml)")
	)
	flag.BoolVar(flagVersion, "v", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `oml (oh-my-logs) v%s - %s
Fast, keyboard-centric terminal log viewer for embedded systems.

Usage:
  oml [flags]                      Launch interactive log viewer
  oml profile <command> [options]  Manage parser profiles & companion decoders

Stream Sources (mutually exclusive):
  --port <device>       Serial port device (e.g. /dev/ttyACM0 or COM3)
  --baud <rate>         Serial baud rate (default: 115200)
  --file <path>         Replay a saved log file instead of a serial port
  --cmd, --exec <cmd>   Run external shell command as live log stream
  --stdin               Read log stream from standard input

Configuration & Profiles:
  --profile <name|path> Active profile (name or .yaml path)
  oml profile list      List installed & local profiles (# ID, version, decoders)
  oml profile install   Install profile/bundle from file, folder, URL, or git repo
  oml profile update    Check for and install profile & decoder updates
  oml profile show      Inspect profile parser, columns, and decoders
  oml profile uninstall Remove installed profile and companion decoders

Logging & Recording:
  --tee <file>          Stream raw incoming lines directly to specified log file
  --direct-to-disk      Enable continuous streaming directly to disk
  --prefix <name>       Filename prefix for direct-to-disk logs (default: oml)

App Management:
  --update              Check for and install latest oml release
  --nightly             Use nightly build channel for updates
  --install             Install oml binary into system/user PATH
  --uninstall           Uninstall oml binary from system/user PATH
  -v, --version         Print version and exit
  -h, --help            Show this help message

For profile management commands and examples:
  oml profile --help
`, version, codename)
	}

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

	if *flagImportProfile == "" && *flagInstallProfile != "" {
		*flagImportProfile = *flagInstallProfile
	}
	if *flagUninstallProfile == "" && *flagRemoveProfile != "" {
		*flagUninstallProfile = *flagRemoveProfile
	}

	if *flagListProfiles {
		os.Exit(cmdProfileList(appCfg))
	}

	if *flagUninstallProfile != "" {
		os.Exit(cmdProfileUninstall(appCfg, []string{*flagUninstallProfile}))
	}

	if *flagImportProfile != "" {
		os.Exit(cmdProfileInstall(appCfg, []string{"--force", *flagImportProfile}))
	}

	if *flagExportProfile != "" {
		args := []string{*flagExportProfile}
		if *flagOut != "" {
			args = append(args, "--out", *flagOut)
		}
		os.Exit(cmdProfileExport(appCfg, args))
	}

	savedSettings, _ := appCfg.LoadSettings()

	// ── Project-local config (.oml.yaml) ─────────────────────────────────────
	projectCfg, _ := config.FindProjectConfig("")

	// ── Resolve profile ───────────────────────────────────────────────────────
	targetProfile := *flagProfile
	if targetProfile == "" && projectCfg != nil && projectCfg.Profile != "" {
		targetProfile = projectCfg.Profile
	}
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
	} else if projectCfg != nil && projectCfg.Baud > 0 {
		serialCfg.Baud = projectCfg.Baud
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
	} else if projectCfg != nil && projectCfg.Port != "" {
		serialCfg.Port = projectCfg.Port
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

	if projectCfg != nil {
		if len(projectCfg.Decoders) > 0 {
			model.SetProjectDecoders(projectCfg.Decoders)
		}
		if projectCfg.StripPrefix != "" {
			model.SetProjectStripPrefix(projectCfg.StripPrefix)
		}
	}

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

	finalModel, err := prog.Run()
	if m, ok := finalModel.(tui.Model); ok {
		_ = m.Close()
	}
	if err != nil {
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
