package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

func runProfileCommand(appCfg *config.AppConfig, args []string) int {
	if len(args) == 0 {
		return cmdProfileList(appCfg)
	}

	sub := strings.ToLower(args[0])
	subArgs := args[1:]

	switch sub {
	case "list", "ls":
		return cmdProfileList(appCfg)
	case "install", "add", "import":
		return cmdProfileInstall(appCfg, subArgs)
	case "update", "upgrade":
		return cmdProfileUpdate(appCfg, subArgs)
	case "uninstall", "remove", "rm", "delete":
		return cmdProfileUninstall(appCfg, subArgs)
	case "show", "info", "inspect":
		return cmdProfileShow(appCfg, subArgs)
	case "export":
		return cmdProfileExport(appCfg, subArgs)
	case "path", "dir":
		return cmdProfilePath(appCfg)
	case "help", "--help", "-h":
		printProfileHelp()
		return 0
	default:
		// Check if user ran `oml profile <id-or-name>` directly as a shortcut to show
		if _, err := config.ResolveProfile(appCfg, args[0]); err == nil {
			return cmdProfileShow(appCfg, args)
		}
		fmt.Fprintf(os.Stderr, "unknown profile subcommand: %q\n\n", args[0])
		printProfileHelp()
		return 1
	}
}

func printProfileHelp() {
	fmt.Println(`oml profile — Manage firmware parser profiles and companion decoders

Usage:
  oml profile [command] [options]

Commands:
  list                          List all available profiles (# ID, version, scope, parser, decoders)
  install <source...>           Install profile(s) from local file, directory, HTTP URL, or Git repo
  update [id|name...]           Check for and install updates from profile origin URL or Git repo
  uninstall <id|name...>        Remove globally installed profile(s) and companion decoders
  show <id|name>                Inspect details of a profile (version, parser, columns, decoders)
  export <id|name> [--out file] Export profile YAML to stdout or file
  path                          Print global profiles and companion decoders directory paths

Options for install / update:
  -f, --force                   Overwrite / re-download profile even if versions match
  -t, --token <token>           Authentication bearer token for private HTTP/Enterprise URLs
  --check                       Dry-run check for available updates without modifying files

Options for uninstall:
  -f, --force                   Skip confirmations

Examples:
  oml profile list
  oml profile install ./profiles/zephyr.yaml
  oml profile install https://raw.githubusercontent.com/.../appliance.yaml
  oml profile install git@github.whirlpool.com:firmware/tools.git
  oml profile update
  oml profile update appliance --check
  oml profile show 1
  oml profile uninstall 1`)
}

func cmdProfileList(appCfg *config.AppConfig) int {
	list := config.ListProfiles(appCfg)
	if len(list) == 0 {
		fmt.Println("No profiles found.")
		if appCfg != nil {
			fmt.Printf("Global profiles directory: %s\n", appCfg.ProfilesDir)
		}
		return 0
	}

	fmt.Printf("Profiles (%d available):\n", len(list))
	fmt.Printf("  %-3s  %-16s %-9s %-10s %-8s %-8s %s\n", "#", "NAME", "VERSION", "SCOPE", "PARSER", "DECODERS", "LOCATION")
	fmt.Printf("  %-3s  %-16s %-9s %-10s %-8s %-8s %s\n", "---", "----", "-------", "-----", "------", "--------", "--------")

	for _, p := range list {
		scopeTag := fmt.Sprintf("[%s]", p.Scope)
		decStr := fmt.Sprintf("%d", p.DecodersCount)
		if p.HasCompanion {
			decStr += " (bundle)"
		}
		verStr := p.Version
		if verStr == "" {
			verStr = "—"
		} else if !strings.HasPrefix(verStr, "v") {
			verStr = "v" + verStr
		}
		fmt.Printf("  %-3d  %-16s %-9s %-10s %-8s %-8s %s\n", p.ID, p.Name, verStr, scopeTag, p.ParserType, decStr, p.Path)
	}

	if appCfg != nil {
		fmt.Printf("\nGlobal profiles: %s\n", appCfg.ProfilesDir)
		if appCfg.DecodersDir != "" {
			fmt.Printf("Global decoders: %s\n", appCfg.DecodersDir)
		}
	}
	return 0
}

func cmdProfileUpdate(appCfg *config.AppConfig, args []string) int {
	var checkOnly bool
	var force bool
	var token string
	var targets []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			checkOnly = true
		case "--force", "-f":
			force = true
		case "--token", "-t":
			if i+1 < len(args) {
				token = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: oml profile update [id|name...] [--check] [-f|--force] [-t|--token <token>]")
			return 0
		default:
			if !strings.HasPrefix(args[i], "-") {
				targets = append(targets, args[i])
			}
		}
	}

	opts := config.InstallOptions{
		Force: force,
		Token: token,
	}

	// 1. If checking updates only
	if checkOnly {
		list := config.ListProfiles(appCfg)
		if len(targets) > 0 {
			var filtered []config.ProfileInfo
			for _, t := range targets {
				if info, err := config.ResolveProfile(appCfg, t); err == nil {
					filtered = append(filtered, *info)
				}
			}
			list = filtered
		}

		fmt.Println("Checking for profile updates...")
		hasAny := false
		for _, p := range list {
			if !p.IsGlobal || p.Source == "" {
				continue
			}
			chk, err := config.CheckProfileUpdate(appCfg, p.Name, opts)
			if err != nil {
				fmt.Printf("  • %-16s [error: %v]\n", p.Name, err)
				continue
			}
			if chk.HasUpdate {
				hasAny = true
				fmt.Printf("  • %-16s update available: %s -> %s (%s)\n", chk.Name, chk.CurrentVersion, chk.RemoteVersion, chk.Source)
			} else {
				fmt.Printf("  • %-16s up to date (%s)\n", chk.Name, chk.CurrentVersion)
			}
		}
		if !hasAny {
			fmt.Println("\nAll configured profiles are up to date.")
		}
		return 0
	}

	// 2. Perform updates
	if len(targets) == 0 {
		// Update all global profiles with a configured source
		results, err := config.UpdateAllProfiles(appCfg, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error updating profiles: %v\n", err)
			return 1
		}
		if len(results) == 0 {
			fmt.Println("No global profiles with configured remote sources found.")
			fmt.Println("Install a profile with a remote URL or Git repo first:")
			fmt.Println("  oml profile install https://.../profile.yaml")
			return 0
		}
		for _, r := range results {
			if r.Error != nil {
				fmt.Fprintf(os.Stderr, "✗ Failed to update %q: %v\n", r.Name, r.Error)
				continue
			}
			extra := ""
			if r.DecodersUpdated > 0 {
				extra = fmt.Sprintf(" (%d decoders updated)", r.DecodersUpdated)
			}
			if r.PreviousVersion != r.NewVersion {
				fmt.Printf("✓ Updated %q: %s -> %s%s\n", r.Name, r.PreviousVersion, r.NewVersion, extra)
			} else {
				fmt.Printf("✓ %q is already up to date (%s)%s\n", r.Name, r.NewVersion, extra)
			}
		}
		return 0
	}

	// Update specific targets
	for _, target := range targets {
		r, err := config.UpdateProfile(appCfg, target, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to update %q: %v\n", target, err)
			return 1
		}
		extra := ""
		if r.DecodersUpdated > 0 {
			extra = fmt.Sprintf(" (%d decoders updated)", r.DecodersUpdated)
		}
		if r.PreviousVersion != r.NewVersion {
			fmt.Printf("✓ Updated %q: %s -> %s%s\n", r.Name, r.PreviousVersion, r.NewVersion, extra)
		} else {
			fmt.Printf("✓ %q is already up to date (%s)%s\n", r.Name, r.NewVersion, extra)
		}
	}

	return 0
}

func cmdProfileInstall(appCfg *config.AppConfig, args []string) int {
	var force bool
	var token string
	var sources []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force", "-f":
			force = true
		case "--token", "-t":
			if i+1 < len(args) {
				token = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: oml profile install [-f|--force] [-t|--token <token>] <source...>")
			return 0
		default:
			if !strings.HasPrefix(args[i], "-") {
				sources = append(sources, args[i])
			}
		}
	}

	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one source file, directory, URL, or git repo required")
		fmt.Fprintln(os.Stderr, "usage: oml profile install [-f] [-t token] <source...>")
		return 1
	}

	opts := config.InstallOptions{
		Force: force,
		Token: token,
	}

	var allResults []config.InstallResult
	for _, src := range sources {
		res, err := config.InstallSource(appCfg, src, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error installing from %q: %v\n", src, err)
			return 1
		}
		allResults = append(allResults, res...)
	}

	if len(allResults) == 1 {
		r := allResults[0]
		action := "installed"
		if r.Overwritten {
			action = "updated"
		}
		fmt.Printf("✓ Successfully %s profile %q to:\n  %s\n", action, r.Name, r.TargetPath)
		if r.CompanionDecoders != "" {
			fmt.Printf("  Companion decoders installed (%d files) in:\n  %s\n", r.DecodersInstalled, r.CompanionDecoders)
		}
		fmt.Printf("\nYou can now run: oml --profile %s\n", r.Name)
	} else if len(allResults) > 1 {
		fmt.Printf("✓ Successfully installed %d profiles:\n", len(allResults))
		for _, r := range allResults {
			extra := ""
			if r.DecodersInstalled > 0 {
				extra = fmt.Sprintf(" [%d decoders]", r.DecodersInstalled)
			}
			fmt.Printf("  • %-16s %s%s\n", r.Name, r.TargetPath, extra)
		}
	}

	return 0
}

func cmdProfileUninstall(appCfg *config.AppConfig, args []string) int {
	var force bool
	var targets []string

	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			fmt.Println("Usage: oml profile uninstall [-f|--force] <id|name...>")
			return 0
		default:
			if !strings.HasPrefix(arg, "-") {
				targets = append(targets, arg)
			}
		}
	}
	_ = force

	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "error: specify at least one profile ID (#1) or name to uninstall")
		fmt.Fprintln(os.Stderr, "usage: oml profile uninstall [-f] <id|name...>")
		return 1
	}

	for _, target := range targets {
		info, err := config.UninstallProfile(appCfg, target, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error uninstalling %q: %v\n", target, err)
			return 1
		}
		fmt.Printf("✓ Successfully uninstalled profile %q (%s)\n", info.Name, info.Path)
	}

	return 0
}

func cmdProfileShow(appCfg *config.AppConfig, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: specify profile ID (#1) or name to show")
		fmt.Fprintln(os.Stderr, "usage: oml profile show <id|name>")
		return 1
	}

	insp, err := config.InspectProfile(appCfg, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	p := insp.Profile
	verStr := p.Version
	if verStr == "" {
		verStr = "unversioned"
	}
	fmt.Printf("Profile: %s (ID: #%d, Version: %s)\n", insp.Info.Name, insp.Info.ID, verStr)
	fmt.Printf("  Scope:    %s\n", insp.Info.Scope)
	if p.Source != "" {
		fmt.Printf("  Source:   %s\n", p.Source)
	}
	fmt.Printf("  Path:     %s\n", insp.FilePath)
	fmt.Printf("  Parser:   %s", p.Parser.Type)
	if p.Parser.Pattern != "" {
		fmt.Printf(" (pattern: %s)", p.Parser.Pattern)
	}
	fmt.Println()

	if len(p.Columns) > 0 {
		fmt.Printf("\nColumns (%d):\n", len(p.Columns))
		for idx, col := range p.Columns {
			widthStr := fmt.Sprintf("%d", col.Width)
			if col.Width == 0 {
				widthStr = "expand"
			}
			fmt.Printf("  %d. %-14s (width: %-6s, title: %q)\n", idx+1, col.Field, widthStr, col.Title)
		}
	}

	if len(p.Decoders) > 0 {
		fmt.Printf("\nPayload Decoders (%d):\n", len(p.Decoders))
		for idx, d := range p.Decoders {
			if d.Exec != "" {
				fmt.Printf("  %d. [exec] match: %-25s command: %s\n", idx+1, d.Match, d.Exec)
			} else {
				fmt.Printf("  %d. [text] match: %-25s format: %s\n", idx+1, d.Match, d.Format)
			}
		}
	}

	return 0
}

func cmdProfileExport(appCfg *config.AppConfig, args []string) int {
	var outPath string
	var target string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out", "-o":
			if i+1 < len(args) {
				outPath = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: oml profile export <id|name> [--out <file>]")
			return 0
		default:
			if !strings.HasPrefix(args[i], "-") && target == "" {
				target = args[i]
			}
		}
	}

	if target == "" {
		fmt.Fprintln(os.Stderr, "error: specify profile ID (#1) or name to export")
		fmt.Fprintln(os.Stderr, "usage: oml profile export <id|name> [--out file]")
		return 1
	}

	content, sourcePath, err := config.ExportProfile(appCfg, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error exporting profile: %v\n", err)
		return 1
	}

	if outPath != "" {
		if err := os.WriteFile(outPath, content, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing to %q: %v\n", outPath, err)
			return 1
		}
		fmt.Printf("✓ Exported profile %q from %s to %s\n", target, sourcePath, outPath)
	} else {
		os.Stdout.Write(content)
	}

	return 0
}

func cmdProfilePath(appCfg *config.AppConfig) int {
	if appCfg != nil {
		fmt.Printf("Profiles: %s\n", appCfg.ProfilesDir)
		if appCfg.DecodersDir != "" {
			fmt.Printf("Decoders: %s\n", appCfg.DecodersDir)
		}
	}
	return 0
}
