package main

import "os"

var (
	version      = "dev"
	forceBackups bool
	onlyStores   []string
)

func main() {
	// Git-style: hand off to an external `store-<cmd>` (or a known
	// companion binary like `stock`) before cobra sees the args.
	if len(os.Args) >= 2 {
		if path, ok := resolveCompanion(os.Args[1]); ok {
			runCompanion(path, os.Args[2:])
			return // runCompanion exits directly; unreachable
		}
	}
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
