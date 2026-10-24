package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "portfull:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "list", "ls":
		if len(args) > 1 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Println("Usage: portfull list\n\nShow localhost ports that are currently listening.")
			return nil
		}
		if len(args) != 1 {
			return errors.New("list does not take arguments; run 'portfull help'")
		}
		return listPorts()
	case "kill", "stop":
		return killPort(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version", "--version":
		fmt.Println("portfull", version)
		return nil
	default:
		return fmt.Errorf("unknown command %q; run 'portfull help'", args[0])
	}
}

func printUsage() {
	fmt.Print(`portfull — find and stop local port listeners

Usage:
  portfull list                 Show listening localhost ports
  portfull kill <port>          Inspect a port, then confirm before stopping it
  portfull kill <port> --yes    Stop without the confirmation prompt
  portfull help                 Show this help

Examples:
  portfull list
  portfull kill 3000
`)
}

func listPorts() error {
	listeners, err := findListeners()
	if err != nil {
		return err
	}
	if len(listeners) == 0 {
		fmt.Println("No listening ports found on localhost.")
		return nil
	}
	fmt.Printf("%-8s %-24s %-9s %s\n", "PORT", "ADDRESS", "PID", "PROCESS")
	for _, listener := range listeners {
		fmt.Printf("%-8d %-24s %-9d %s\n", listener.Port, listener.Address, listener.PID, listener.Command)
	}
	fmt.Printf("\n%d listening port(s)\n", len(listeners))
	return nil
}

func killPort(args []string) error {
	yes := false
	filtered := args[:0]
	for _, arg := range args {
		if arg == "--yes" || arg == "-y" {
			yes = true
			continue
		}
		filtered = append(filtered, arg)
	}
	fs := flag.NewFlagSet("kill", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(filtered); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: portfull kill <port> [--yes]")
	}
	port, err := strconv.Atoi(fs.Arg(0))
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%q is not a valid TCP port (1–65535)", fs.Arg(0))
	}
	listeners, err := findListeners()
	if err != nil {
		return err
	}
	var matches []Listener
	for _, listener := range listeners {
		if listener.Port == port {
			matches = append(matches, listener)
		}
	}
	if len(matches) == 0 {
		fmt.Printf("Port %d is free. Nothing to stop.\n", port)
		return nil
	}
	seen := make(map[int]bool)
	for _, listener := range matches {
		fmt.Printf("Port %d is in use by %s (PID %d) on %s.\n", port, listener.Command, listener.PID, listener.Address)
		seen[listener.PID] = true
	}
	if !yes {
		if !isTerminal(os.Stdin) {
			return errors.New("confirmation needs a terminal; rerun with --yes to stop the process")
		}
		fmt.Print("Stop this process? [y/N] ")
		answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && len(answer) == 0 {
			fmt.Println("Skipped.")
			return nil
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Skipped. The process is still running.")
			return nil
		}
	}
	for pid := range seen {
		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not find PID %d: %v\n", pid, err)
			continue
		}
		if err := process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Could not stop PID %d: %v\n", pid, err)
			continue
		}
		fmt.Printf("Sent stop signal to PID %d.\n", pid)
	}
	return nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

// Linux uses /proc to discover listening sockets and their owning processes.
func findListeners() ([]Listener, error) {
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		return nil, errors.New("portfull currently requires Linux with /proc mounted")
	}
	return linuxListeners()
}
