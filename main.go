package main

import (
	"ipcrawler/cmd"
	"ipcrawler/internal/ui"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		ui.Global.Messages.Printf(ui.ErrorStyle.Render(ui.ErrorPrefix+" Application failed: %v")+"\n", err)
		os.Exit(1)
	}
}