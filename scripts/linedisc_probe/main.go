// Command linedisc_probe prints what SerialSource emits for a serial port.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func main() {
	src, err := serial.NewSerialSource(serial.Config{Port: os.Args[1], Baud: 115200})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer src.Stop()
	t0 := time.Now()
	for l := range src.Lines() {
		fmt.Printf("%6.2fs %q\n", time.Since(t0).Seconds(), l)
	}
}
