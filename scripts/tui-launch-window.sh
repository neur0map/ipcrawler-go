#!/bin/bash

# IPCrawler TUI - Cross-Platform New Terminal Window Opener
# This script detects the OS and opens the TUI in a new terminal window
# Supports: macOS, Linux (various DEs), Windows (WSL/Git Bash/Cygwin), FreeBSD

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/../bin/ipcrawler"

# Check for dry-run mode
DRY_RUN=false
if [[ "$1" == "--dry-run" || "$1" == "-n" ]]; then
    DRY_RUN=true
    echo "🧪 DRY RUN MODE - No terminal will be opened"
fi

echo "🚀 Opening IPCrawler TUI in NEW terminal window..."

# Check if binary exists
if [ ! -f "$BINARY" ]; then
    echo "❌ Binary not found: $BINARY"
    echo "Run 'make build' first"
    exit 1
fi

# Enhanced OS detection
detect_os() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "linux"
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
        echo "windows"
    elif [[ "$OSTYPE" == "freebsd"* ]]; then
        echo "freebsd"
    elif command -v wsl.exe >/dev/null 2>&1; then
        echo "wsl"
    elif [[ -n "$WSL_DISTRO_NAME" ]]; then
        echo "wsl"
    else
        echo "unknown"
    fi
}

# Create a proper launcher that will run in the new terminal
LAUNCHER="/tmp/ipcrawler_new_window_$$.sh"
cat > "$LAUNCHER" << 'LAUNCHER_EOF'
#!/bin/bash

# Set the terminal title and size
echo -e "\033]0;IPCrawler TUI Dashboard\007"  # Set title
printf '\e[8;70;200t'  # Resize to 70 rows, 200 columns

clear
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                   IPCrawler TUI Dashboard                    ║" 
echo "║                    NEW TERMINAL WINDOW                       ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  Window Size: 200x70 (optimized for horizontal card layout) ║"
echo "║  Features: 4 cards horizontal + full-width output           ║"
echo "║  Controls: Tab=cycle cards, 1-4=direct focus, q=quit        ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Starting TUI in 3 seconds..."
sleep 1 && echo "2..." && sleep 1 && echo "1..." && sleep 1

# Execute the TUI
LAUNCHER_EOF

# Add the actual execution line with the binary path
echo "exec \"$BINARY\" --new-window" >> "$LAUNCHER"

chmod +x "$LAUNCHER"

# Detect current operating system
OS=$(detect_os)
echo "🔍 Detected OS: $OS"

# Platform-specific new terminal window opening
case "$OS" in
    "macos")
        echo "📱 macOS detected - trying various terminal applications"
        
        # Method 1: AppleScript for Terminal.app (most reliable)
        echo "   Trying: Terminal.app via AppleScript"
        if [[ "$DRY_RUN" == "true" ]]; then
            echo "   [DRY RUN] Would execute: osascript -e \"tell application \\\"Terminal\\\" to do script \\\"bash '$LAUNCHER'; exit\\\"\""
            echo "✅ [DRY RUN] Success: Would open new Terminal.app window"
            exit 0
        elif osascript -e "tell application \"Terminal\" to do script \"bash '$LAUNCHER'; exit\"" >/dev/null 2>&1; then
            echo "✅ Success: New Terminal.app window opened"
            exit 0
        fi
        
        # Method 2: iTerm2 if available and running
        if pgrep -x "iTerm2" > /dev/null; then
            echo "   Trying: iTerm2 (currently running)"
            if osascript -e "tell application \"iTerm\" to create window with default profile command \"bash '$LAUNCHER'; exit\"" >/dev/null 2>&1; then
                echo "✅ Success: New iTerm2 window opened"  
                exit 0
            fi
        elif [ -d "/Applications/iTerm.app" ]; then
            echo "   Trying: iTerm2 (available but not running)"
            if osascript -e "tell application \"iTerm\" to create window with default profile command \"bash '$LAUNCHER'; exit\"" >/dev/null 2>&1; then
                echo "✅ Success: New iTerm2 window opened"  
                exit 0
            fi
        fi
        
        # Method 3: Use 'open' command as fallback
        echo "   Trying: open command with Terminal.app"
        if open -a Terminal "$LAUNCHER" 2>/dev/null; then
            echo "✅ Success: Terminal opened via 'open' command"
            exit 0
        fi
        ;;
        
    "linux")
        echo "🐧 Linux detected - trying desktop environment terminals"
        
        # Detect desktop environment for better terminal selection
        if [[ "$XDG_CURRENT_DESKTOP" == *"GNOME"* ]] && command -v gnome-terminal >/dev/null 2>&1; then
            echo "   Trying: gnome-terminal (GNOME DE detected)"
            if gnome-terminal --geometry=200x70 --title="IPCrawler TUI" -- bash "$LAUNCHER" 2>/dev/null; then
                echo "✅ Success: New gnome-terminal window"
                exit 0
            fi
        elif [[ "$XDG_CURRENT_DESKTOP" == *"KDE"* ]] && command -v konsole >/dev/null 2>&1; then
            echo "   Trying: konsole (KDE DE detected)"
            konsole --geometry 200x70 --title "IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
            if [ $? -eq 0 ]; then
                echo "✅ Success: New konsole window"
                exit 0
            fi
        fi
        
        # Fallback to available terminals
        for terminal in gnome-terminal konsole xfce4-terminal mate-terminal terminator xterm; do
            if command -v "$terminal" >/dev/null 2>&1; then
                echo "   Trying: $terminal"
                case "$terminal" in
                    gnome-terminal)
                        if $terminal --geometry=200x70 --title="IPCrawler TUI" -- bash "$LAUNCHER" 2>/dev/null; then
                            echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    konsole)
                        $terminal --geometry 200x70 --title "IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    xfce4-terminal)
                        $terminal --geometry=200x70 --title="IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    mate-terminal)
                        $terminal --geometry=200x70 --title="IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    terminator)
                        $terminal --geometry=200x70 --title="IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    xterm)
                        $terminal -geometry 200x70 -title "IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                esac
            fi
        done
        ;;
        
    "windows")
        echo "🪟 Windows detected (Git Bash/Cygwin/MSYS2)"
        
        # Try Windows Terminal (modern)
        if command -v wt.exe >/dev/null 2>&1; then
            echo "   Trying: Windows Terminal"
            if wt.exe new-tab --title "IPCrawler TUI" bash "$LAUNCHER" 2>/dev/null; then
                echo "✅ Success: New Windows Terminal tab"
                exit 0
            fi
        fi
        
        # Try CMD with start command
        if command -v cmd.exe >/dev/null 2>&1; then
            echo "   Trying: CMD with start command"
            if cmd.exe /c start "IPCrawler TUI" bash "$LAUNCHER" 2>/dev/null; then
                echo "✅ Success: New CMD window"
                exit 0
            fi
        fi
        
        # Fallback for Git Bash
        echo "   Trying: Git Bash new window"
        if start bash "$LAUNCHER" 2>/dev/null; then
            echo "✅ Success: New Git Bash window"
            exit 0
        fi
        ;;
        
    "wsl")
        echo "🐧🪟 WSL detected - using Windows host terminal"
        
        # Try Windows Terminal from WSL
        if command -v wt.exe >/dev/null 2>&1; then
            echo "   Trying: Windows Terminal from WSL"
            if wt.exe new-tab --title "IPCrawler TUI" wsl bash "$LAUNCHER" 2>/dev/null; then
                echo "✅ Success: New Windows Terminal with WSL"
                exit 0
            fi
        fi
        
        # Fallback to PowerShell/CMD
        if command -v powershell.exe >/dev/null 2>&1; then
            echo "   Trying: PowerShell from WSL"
            if powershell.exe -Command "Start-Process wsl -ArgumentList 'bash $LAUNCHER' -WindowStyle Normal" 2>/dev/null; then
                echo "✅ Success: New PowerShell window with WSL"
                exit 0
            fi
        fi
        ;;
        
    "freebsd")
        echo "😈 FreeBSD detected"
        
        # Try common FreeBSD terminals
        for terminal in gnome-terminal konsole xterm; do
            if command -v "$terminal" >/dev/null 2>&1; then
                echo "   Trying: $terminal"
                case "$terminal" in
                    gnome-terminal)
                        if $terminal --geometry=200x70 --title="IPCrawler TUI" -- bash "$LAUNCHER" 2>/dev/null; then
                            echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    konsole)
                        $terminal --geometry 200x70 --title "IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                    xterm)
                        $terminal -geometry 200x70 -title "IPCrawler TUI" -e bash "$LAUNCHER" 2>/dev/null &
                        if [ $? -eq 0 ]; then echo "✅ Success: New $terminal window"; exit 0; fi ;;
                esac
            fi
        done
        ;;
        
    *)
        echo "❓ Unknown OS detected: $OSTYPE"
        ;;
esac

# If all methods failed
echo "❌ Could not open new terminal window"
echo ""
echo "💡 Manual alternative:"
echo "   1. Open a new terminal window manually"
echo "   2. Resize it to 200x70"
echo "   3. Run: $BINARY --new-window"
echo ""
echo "📖 See RESIZE_GUIDE.md for platform-specific instructions"

# Clean up
rm -f "$LAUNCHER"
exit 1