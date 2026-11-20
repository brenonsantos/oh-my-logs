// Command oml-sim runs a standalone, hardware-free embedded UART simulator on a
// virtual serial port (PTY) so oh-my-logs can be tested without physical boards.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/sim"
)

func main() {
	var (
		flagProfile  = flag.String("profile", "zephyr", "device personality: zephyr, esp-idf, linux")
		flagPrompt   = flag.String("prompt", "", "override shell prompt (e.g. 'device:~$ ')")
		flagScenario = flag.String("scenario", "all", "traffic scenario: normal, vt-stress, flood, payloads, all")
		flagRate     = flag.Float64("rate", 1.5, "background async log rate in Hz (0 = quiet/shell only)")
		flagPause    = flag.Duration("mid-line-pause", 50*time.Millisecond, "pause injected mid-line during VT stress")
		flagBurst    = flag.Int("burst", 0, "optional startup burst count (e.g. 10000)")
		flagNoColor  = flag.Bool("no-color", false, "disable ANSI SGR colors in log output")
		flagSymlink  = flag.String("symlink", "/tmp/oml-sim", "convenience symlink path pointing to the PTY (empty to disable)")
		flagSeed     = flag.Int64("seed", 0, "RNG seed for reproducible logs (0 = random)")
	)
	flag.Parse()

	profile, err := sim.ParseProfile(*flagProfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *flagPrompt != "" {
		profile.Prompt = *flagPrompt
	}
	if *flagNoColor {
		profile.UseColor = false
	}

	scenario, err := sim.ParseScenario(*flagScenario)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	pty, err := sim.OpenPTY(*flagSymlink)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() {
		_ = pty.Close()
	}()

	connectPort := pty.SlavePath
	if pty.Symlink != "" {
		connectPort = pty.Symlink
	}

	fmt.Printf("oml-sim running\n")
	fmt.Printf("  PTY:      %s\n", pty.SlavePath)
	if pty.Symlink != "" {
		fmt.Printf("  Symlink:  %s\n", pty.Symlink)
	}
	fmt.Printf("  Profile:  %s (prompt %q)\n", profile.Kind, profile.Prompt)
	fmt.Printf("  Scenario: %s (rate=%.2f Hz, mid-line-pause=%v)\n", scenario, *flagRate, *flagPause)
	fmt.Printf("\nConnect with:\n  go run ./cmd/oml --port %s", connectPort)
	if profile.Kind == sim.ProfileZephyr {
		fmt.Printf(" --profile zephyr")
	}
	fmt.Printf("\n\nPress Ctrl+C to stop.\n")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		_ = pty.Close()
	}()

	dev := sim.NewDevice(pty, sim.Config{
		Profile:      profile,
		Scenario:     scenario,
		RateHz:       *flagRate,
		MidLinePause: *flagPause,
		StartupBurst: *flagBurst,
		Seed:         *flagSeed,
	})

	if err := dev.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "oml-sim stopped: %v\n", err)
		os.Exit(1)
	}
}
