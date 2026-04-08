package main

import "os"

var (
	version      = "dev"
	forceBackups bool
	onlyStores   []string
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
