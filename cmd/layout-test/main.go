package main

import (
	"fmt"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui"
)

// layoutTestModel wraps the TUI model for testing different sizes
type layoutTestModel struct {
	tui      ui.Model
	testSizes []testSize
	current   int
	autoMode  bool
}

type testSize struct {
	width, height int
	name          string
}

func (m layoutTestModel) Init() tea.Cmd {
	return m.tui.Init()
}

func (m layoutTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n", "right":
			// Next size
			if m.current < len(m.testSizes)-1 {
				m.current++
				return m.resizeToCurrentTest(), nil
			}
		case "p", "left":
			// Previous size
			if m.current > 0 {
				m.current--
				return m.resizeToCurrentTest(), nil
			}
		case "a":
			// Toggle auto mode
			m.autoMode = !m.autoMode
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// Forward to TUI
		newTUI, cmd := m.tui.Update(msg)
		m.tui = newTUI.(ui.Model)
		return m, cmd
	}

	// Forward other messages to TUI
	newTUI, cmd := m.tui.Update(msg)
	m.tui = newTUI.(ui.Model)
	return m, cmd
}

func (m layoutTestModel) View() string {
	currentSize := m.testSizes[m.current]
	
	header := fmt.Sprintf("Layout Test - %s (%dx%d) - [%d/%d]\n", 
		currentSize.name, currentSize.width, currentSize.height, 
		m.current+1, len(m.testSizes))
	
	controls := "Controls: n/→ next, p/← prev, a auto-mode, q quit\n\n"
	
	tuiView := m.tui.View()
	
	return header + controls + tuiView
}

func (m layoutTestModel) resizeToCurrentTest() layoutTestModel {
	size := m.testSizes[m.current]
	
	// Create a WindowSizeMsg for the current test size
	msg := tea.WindowSizeMsg{
		Width:  size.width,
		Height: size.height - 3, // Account for our header
	}
	
	newTUI, _ := m.tui.Update(msg)
	m.tui = newTUI.(ui.Model)
	
	return m
}

func main() {
	// Get target from command line or use default
	target := "ipcrawler.io"
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	// Define test sizes covering all layout modes
	testSizes := []testSize{
		{160, 48, "Extra Large (160x48)"},
		{140, 40, "Large+ (140x40)"},
		{120, 30, "Large Min (120x30)"},
		{119, 28, "Medium+ (119x28)"},
		{100, 25, "Medium (100x25)"},
		{90, 24, "Medium- (90x24)"},
		{80, 22, "Medium Min (80x22)"},
		{79, 20, "Small+ (79x20)"},
		{60, 18, "Small (60x18)"},
		{50, 16, "Small- (50x16)"},
		{40, 15, "Small Min (40x15)"},
		{35, 12, "Too Small (35x12)"},
	}

	// Check if specific size requested
	if len(os.Args) >= 3 {
		width, err1 := strconv.Atoi(os.Args[1])
		height, err2 := strconv.Atoi(os.Args[2])
		if err1 == nil && err2 == nil {
			testSizes = []testSize{{width, height, fmt.Sprintf("Custom (%dx%d)", width, height)}}
			target = "custom.test"
		}
	}

	// Create the TUI model
	tuiModel := ui.NewModel(target)
	
	// Create the test wrapper
	model := layoutTestModel{
		tui:       tuiModel,
		testSizes: testSizes,
		current:   0,
		autoMode:  false,
	}
	
	// Start with first test size
	model = model.resizeToCurrentTest()

	fmt.Printf("IPCrawler TUI Layout Test\n")
	fmt.Printf("Target: %s\n", target)
	fmt.Printf("Testing %d different layout sizes\n", len(testSizes))
	fmt.Printf("Use n/p to navigate, q to quit\n\n")

	// Create and run the program
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running layout test: %v\n", err)
		os.Exit(1)
	}
}