package main

import (
	"ipcrawler/cmd"
	"os"
	
	"github.com/pterm/pterm"
)

func main() {
	if err := cmd.Execute(); err != nil {
		pterm.Error.Println("Application failed:", err)
		os.Exit(1)
	}
}