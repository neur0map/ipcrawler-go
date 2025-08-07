package main

import (
	"fmt"
	"os"
	
	"ipcrawler/cmd"
	
	// Import scanners and templates to register them
	_ "ipcrawler/internal/scanners"
	_ "ipcrawler/internal/templates"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Application failed: %v\n", err)
		os.Exit(1)
	}
}