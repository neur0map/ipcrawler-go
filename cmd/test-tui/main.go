package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/carlosm/ipcrawler/internal/ui"
)

func main() {
	var (
		target = flag.String("target", "ipcrawler.io", "Target to scan (for demo)")
		demo   = flag.Bool("demo", false, "Run in demo mode with simulated events")
		debug  = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Set up logging
	if *debug {
		os.Setenv("DEBUG", "1")
		log.SetLevel(log.DebugLevel)
	}

	// Set up signal handling for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	if *demo {
		fmt.Printf("Starting TUI demo for target: %s\n", *target)
		fmt.Println("This will test the new Charmbracelet-based TUI implementation")
		fmt.Println("Use the following keys:")
		fmt.Println("  Tab       - Switch between panels")
		fmt.Println("  ?         - Toggle help")
		fmt.Println("  q/Ctrl+C  - Quit")
		fmt.Println("  ↑/↓ or j/k - Navigate lists")
		fmt.Println()
		fmt.Println("Starting in 2 seconds...")

		// Brief pause to read instructions
		select {
		case <-c:
			fmt.Println("Cancelled before start")
			return
		default:
			// Continue
		}

		// Create and run demo
		demo := ui.NewDemoRunner(*target)
		
		// Handle graceful shutdown
		go func() {
			<-c
			fmt.Println("\nShutting down demo...")
			demo.Quit()
		}()

		if err := demo.RunDemo(); err != nil {
			log.Error("Demo failed", "error", err)
			os.Exit(1)
		}
	} else {
		// Regular TUI mode (without demo events)
		fmt.Printf("Starting TUI for target: %s\n", *target)
		
		runner := ui.NewRunner(*target)
		
		// Handle graceful shutdown
		go func() {
			<-c
			fmt.Println("\nShutting down TUI...")
			runner.Quit()
		}()

		if err := runner.Run(); err != nil {
			log.Error("TUI failed", "error", err)
			os.Exit(1)
		}
	}

	fmt.Println("TUI exited cleanly")
}