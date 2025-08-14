package executor

import (
	"github.com/neur0map/ipcrawler/internal/tools/naabu"
	"github.com/neur0map/ipcrawler/internal/tools/nmap"
)

// RegisterAllParsers registers all available tool output parsers
// This is the ONLY place where tool-specific parsers are imported
// Adding a new tool requires only adding its import and registration here
func RegisterAllParsers(manager *MagicVariableManager) {
	// Register naabu parser
	manager.RegisterParser(&naabu.OutputParser{})
	
	// Register nmap parser
	manager.RegisterParser(&nmap.OutputParser{})

	// Future parsers can be added here:
	// manager.RegisterParser(&subfinder.OutputParser{})
	// manager.RegisterParser(&httpx.OutputParser{})
}