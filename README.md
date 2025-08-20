# IPCrawler 🎯

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey.svg)](https://github.com/neur0map/ipcrawler-go)
[![AI-Assisted](https://img.shields.io/badge/Development-AI--Assisted-purple.svg)](https://claude.ai/code)

> **A Student's Journey into Cybersecurity - Built for Learning and Hack The Box Practice**

Hi! I'm a cybersecurity student trying to break into the cyber industry, and IPCrawler is my hobby project to help me practice penetration testing skills on platforms like Hack The Box. This tool combines my learning journey with practical security testing needs.

**🤖 Transparency:** This project was largely developed with AI assistance (Claude Code) as part of my programming learning process. As a beginner programmer, I used AI to help implement advanced features while I focus on understanding cybersecurity concepts and techniques.

## 🚀 Quick Install

**One-liner installation (recommended):**
```bash
curl -fsSL https://raw.githubusercontent.com/neur0map/ipcrawler-go/main/install.sh | sudo bash
```

**Alternative manual installation:**
```bash
git clone https://github.com/neur0map/ipcrawler-go.git
cd ipcrawler-go
make easy
```

## 🎓 What IPCrawler Does (For Fellow Students)

IPCrawler is basically a "Swiss Army knife" for penetration testing that I built to help with my Hack The Box practice sessions and CTF challenges. Instead of running multiple terminal windows with different commands, everything runs in one clean interface.

### 🖥️ **Cool TUI Interface** 
- Clean dashboard that shows everything happening at once
- No need to juggle multiple terminal windows
- Real-time system monitoring (CPU, memory, etc.)
- Just hit Tab to cycle between different panels
- Saves your target between sessions (super helpful for HTB boxes!)

### ⚡ **Automated Security Scanning**
- **Port Scanning** - Finds open ports on targets (great for HTB enumeration phase)
- **DNS Enumeration** - Discovers subdomains and DNS records
- **Service Detection** - Identifies what's running on those ports
- **Concurrent Execution** - Runs multiple scans at the same time (saves tons of time!)

### 🛠️ **Tools I Integrated** (with AI help!)
- **Nmap** - The classic port scanner everyone uses
- **Naabu** - Super fast port scanner for when Nmap is too slow
- **Nslookup** - DNS queries and enumeration
- **Easy to add more** - YAML config files make it simple

### 📋 **Workflow System** (My Favorite Part!)
Instead of remembering all those command flags and options, I created "workflows" that run common penetration testing sequences:
- **Reconnaissance workflow** - Does the initial target discovery
- **DNS enumeration workflow** - Finds all the DNS info
- **Port scanning workflow** - Comprehensive port analysis

## 🎮 How to Use (Perfect for HTB!)

### Getting Started (Super Easy!)
```bash
# Just run it with your target - works with IPs, domains, whatever
ipcrawler 10.10.10.10          # HTB machine IP
ipcrawler example.com          # Domain name  
ipcrawler 192.168.1.0/24       # Network range

# Need admin privileges for some scans? No problem:
sudo ipcrawler 10.10.10.10
```

### Real HTB Example Workflow
```bash
# Start scanning an HTB machine
ipcrawler 10.10.10.87

# The TUI will pop up and you can:
# 1. Select "Port Scanning" workflow  
# 2. Select "DNS Enumeration" workflow
# 3. Watch them run in real-time
# 4. Check the logs for detailed output
# 5. Results save automatically for your writeups!
```

### Navigation (It's Actually Fun!)
- **Tab** - Jump between different panels
- **Arrow Keys** - Navigate through lists
- **Space/Enter** - Select workflows to run  
- **q** - Quit when done
- **1-5** - Quick jump to specific panels

### Pro Tips for Students
```bash
# See what tools are available
ipcrawler registry list

# Test mode (doesn't actually scan)
ipcrawler --debug target.com

# Old school CLI mode (for scripts)
ipcrawler no-tui target.com
```

## 🏗️ How It Works (For the Curious)

**Full transparency:** Most of the complex architecture was implemented with heavy AI assistance since I'm still learning advanced Go programming. But understanding how it works helps me learn!

### The Main Parts (AI helped me design this!)

1. **The Pretty Interface** (`cmd/ipcrawler/main.go`)
   - Uses Charmbracelet's Bubble Tea framework for the TUI
   - Shows real-time progress of all your scans
   - Handles all the keyboard input and navigation

2. **The Engine** (`internal/executor/`)
   - **engine.go** - Actually runs the security tools
   - **progress.go** - Shows those cool progress indicators 
   - **template_resolver.go** - Replaces `{{target}}` with your actual target
   - **tool_config.go** - Loads settings from YAML files

3. **Configuration** (`configs/` folder)
   - All the settings in easy-to-edit YAML files
   - Security policies to keep things safe
   - UI customization options

4. **Workflows** (`workflows/` folder)
   - Pre-built scanning sequences for common pentesting tasks
   - Easy to modify for your specific needs
   - Organized by category (reconnaissance, enumeration, etc.)

### Cool Features I Learned About
- **Template Variables** - Use `{{target}}` in configs and it gets replaced with your actual target
- **Session Saving** - Remembers your last target when you restart
- **Concurrent Execution** - Runs multiple tools at the same time
- **Safety Checks** - Validates inputs to prevent accidents

## 📁 Customization (Easy to Tweak!)

One of the coolest things about this project is that everything is configurable through simple YAML files. Even as a beginner, I can easily modify how tools behave without touching code!

### Main Settings (`configs/` folder)
```yaml
# ui.yaml - Change colors, themes, performance settings
# security.yaml - Safety limits and validation rules  
# output.yaml - Where logs go and how detailed they are
# tools.yaml - Timeout settings and execution limits
```

### Tool Settings (`tools/` folder)
```yaml
# nmap/config.yaml - All the nmap scanning options
# naabu/config.yaml - Fast port scanner settings
# nslookup/config.yaml - DNS query configurations
# reusable.yaml - Variables you can use everywhere
```

### Workflow Definitions (`workflows/` folder)
```yaml
# reconnaissance/ - Basic target discovery workflows
# DNS-Scanning/ - DNS enumeration sequences  
# descriptions.yaml - What each workflow does
```

### Want to Add a New Tool?
1. Create `tools/newtool/config.yaml` 
2. Define how it should run
3. Add it to a workflow
4. Done! (This was way easier than I expected thanks to the AI design)

## 🔨 Development (Learning Journey)

As a student learning to code, I found these commands super helpful for building and testing:

### Build Commands (AI taught me these!)
```bash
make build     # Compile the Go code into a binary
make run       # Launch in a new terminal window  
make dev       # Auto-rebuild when files change (great for learning!)
make easy      # Create global command (what most people want)
```

### Testing (Important for Learning!)
```bash
make test-all      # Run all the tests  
make test-static   # Make sure the TUI architecture is correct
make test-deps     # Verify all dependencies work
```

### Learning by Adding Tools
This is actually how I'm learning more about penetration testing - by adding new tools:

1. Study how existing tools work in `tools/` folder
2. Create `tools/newtool/config.yaml` for your new tool
3. Add it to a workflow in `workflows/`
4. Test it on HTB machines!

**Pro tip:** Start by modifying existing tool configs to understand the pattern, then create your own.

## 🛡️ Security Features

### Execution Validation
- **Path Validation** - Prevents directory traversal attacks
- **Argument Sanitization** - Configurable character class filtering
- **Executable Verification** - Tools must reside under `tools_path`
- **Shell Metacharacter Filtering** - Prevents command injection

### Privilege Management
- **Automatic Sudo Detection** - Preserves privileges when needed
- **Sudoers Configuration** - Safe elevated execution for security tools
- **Environment Preservation** - Maintains user context

## 📋 System Requirements

### Supported Platforms
- **Linux** (Ubuntu, Debian, RHEL, Fedora, Arch, Alpine, SUSE)
- **macOS** (with Homebrew)
- **Architecture** (amd64, arm64, 386, arm)

### Dependencies
- **Go 1.19+** (automatically installed)
- **Git** (for repository cloning)
- **Make** (for build automation)
- **Security Tools** (nmap, nslookup, naabu - auto-installed)

### Optional Tools
- **Curl/Wget** (for downloads)
- **Terminal** (200x70 optimal size)

## 🔧 Installation Details

The installer performs:
1. **System Detection** - OS, distribution, and architecture
2. **Dependency Installation** - Go toolchain and security tools  
3. **Global Installation** - Complete project copy to `/opt/ipcrawler`
4. **Wrapper Script Creation** - Smart wrapper that sets correct working directory
5. **Permission Configuration** - Sudo and normal user execution
6. **Verification** - Functionality and tool availability testing

### Installation Structure
```
/opt/ipcrawler/           # Full project installation
├── bin/ipcrawler         # Actual binary
├── workflows/            # YAML workflow definitions
├── configs/              # Configuration files
├── tools/                # Tool configurations
└── local_files/logs/     # Runtime logs

/usr/local/bin/ipcrawler       # Wrapper script (main command)
/usr/local/bin/ipcrawler-direct # Direct binary symlink  
/usr/local/bin/ipcrawler-sudo   # Auto-sudo wrapper
```

The wrapper script automatically changes to `/opt/ipcrawler` before executing the binary, ensuring access to all workflow files, configurations, and tool definitions regardless of where the command is run from.

### Manual Installation
```bash
# Clone repository
git clone https://github.com/neur0map/ipcrawler-go.git
cd ipcrawler-go

# Build and install
make deps
make build  
make easy   # Creates global symlink
```

## 📊 Performance

### Optimization Features
- **Framerate Capping** - Configurable UI refresh limits
- **Lazy Rendering** - Reduced CPU usage in idle state
- **Viewport Optimization** - Efficient text handling
- **Concurrent Execution Limits** - Configurable parallelism

### Monitoring
- **Real-time Metrics** - CPU, memory, network usage
- **Execution Tracking** - PTerm-based progress indicators
- **Log Analysis** - Structured logging with levels
- **Resource Usage** - System performance monitoring

## 🤝 Contributing (Fellow Students Welcome!)

This project is perfect for other cybersecurity students who want to contribute! Since most of the complex stuff was AI-assisted, you can focus on the security aspects rather than advanced programming.

### Easy Ways to Contribute
1. **Add New Security Tools** - Create YAML configs for tools you use in HTB
2. **Create New Workflows** - Share your favorite pentesting sequences  
3. **Improve Documentation** - Help other students understand features
4. **Test on Different Targets** - Try it on HTB machines and report issues
5. **Share HTB Writeups** - Show how you used IPCrawler in your solutions

### How to Contribute
1. Fork the repository (button on GitHub)
2. Create a new branch (`git checkout -b my-htb-improvement`)
3. Make your changes (add tools, workflows, or documentation)
4. Test it on some HTB machines
5. Submit a Pull Request with details about what you added

### What I'm Looking For
- **More security tools integration** (gobuster, dirb, sqlmap, etc.)
- **Better workflows for different attack vectors** 
- **HTB-specific scanning profiles**
- **Beginner-friendly documentation improvements**
- **Bug reports from actual pentesting use**

**Remember:** You don't need to be an expert programmer - focus on the cybersecurity knowledge!

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Links

- **Repository**: https://github.com/neur0map/ipcrawler-go
- **Issues**: https://github.com/neur0map/ipcrawler-go/issues
- **Documentation**: See `CLAUDE.md` for detailed development guide
- **Security Policy**: Follow responsible disclosure practices

## 🙏 Acknowledgments & Learning Credits

### AI Development Partner
- **Claude Code by Anthropic** - This project was built with heavy AI assistance. As a cybersecurity student, I focused on learning the security concepts while Claude Code helped implement the complex programming architecture, concurrency management, and advanced Go features that were beyond my current skill level.

### Amazing Open Source Tools & Frameworks
- **Charmbracelet** - For the incredible TUI framework ([Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), [Lip Gloss](https://github.com/charmbracelet/lipgloss))
- **PTerm** - For beautiful terminal output management  
- **Security Community** - For all the awesome tools that IPCrawler integrates

### Learning Resources That Helped
- **Hack The Box** - Primary testing ground and inspiration
- **Go Documentation & Community** - Learning the language basics
- **Cybersecurity YouTube/Blog Community** - Understanding pentesting workflows
- **GitHub Open Source Projects** - Learning by reading other people's code

### Fellow Students & Mentors
- Thanks to everyone who tests this tool and provides feedback!
- Special thanks to the cybersecurity student community for sharing knowledge

---

## 🎯 Ready to Start Your Pentesting Journey?

**Install IPCrawler in seconds and start practicing on HTB machines:**

```bash
curl -fsSL https://raw.githubusercontent.com/neur0map/ipcrawler-go/main/install.sh | sudo bash
```

*Built by a student, for students. AI-assisted but security-focused. Perfect for Hack The Box practice!* 🚀