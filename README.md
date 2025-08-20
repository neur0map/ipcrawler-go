# IPCrawler 🎯

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey.svg)](https://github.com/neur0map/ipcrawler-go)
[![AI-Assisted](https://img.shields.io/badge/Development-AI--Assisted-purple.svg)](https://claude.ai/code)

> **A Student's Journey into Cybersecurity - Built for Learning and Hack The Box Practice**

Hi! I'm a cybersecurity student trying to break into the cyber industry, and IPCrawler is my hobby project to help me practice penetration testing skills on platforms like Hack The Box. This tool combines my learning journey with practical security testing needs.

**🤖 Transparency:** This project was largely developed with AI assistance (Claude Code) as part of my programming learning process. As a beginner programmer, I used AI to help implement advanced features while I focus on understanding cybersecurity concepts and techniques.

## 🚀 Installation Options

### 📦 **Production Installation**
**One-liner for ready-to-use IPCrawler:**
```bash
curl -fsSL https://raw.githubusercontent.com/neur0map/ipcrawler-go/main/install.sh | sudo bash
```
- ✅ Automatic system detection and dependency installation
- ✅ Global binary installation with wrapper scripts
- ✅ No build tools required - just download and use
- ✅ Perfect for HTB practice and normal security testing

### 🛠️ **Development Installation**
**Manual setup for building, testing, and contributing:**
```bash
git clone https://github.com/neur0map/ipcrawler-go.git
cd ipcrawler-go
make easy    # Build from source and create global symlink
```
- ✅ Full source code access for learning and modification
- ✅ Make commands for building, testing, and development
- ✅ Perfect for adding new tools, workflows, or contributing
- ✅ Ideal for cybersecurity students who want to understand the code

## 🎓 What IPCrawler Does (For Fellow Students)

IPCrawler is basically a "Swiss Army knife" for penetration testing that I built to help with my Hack The Box practice sessions and CTF challenges. Instead of remembering all those nmap flags and running tools manually, it automatically runs multiple security tools in organized workflows.

### ⚡ **Automated CLI Tool** 
- Runs all security tools automatically with one command
- Clean progress output shows what's happening in real-time
- No need to remember complex command-line flags
- Organized output logs for easy analysis
- Smart workflow execution saves tons of time on HTB!

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
# Start scanning an HTB machine - it runs ALL workflows automatically!
ipcrawler 10.10.10.87

# What happens:
# 1. Automatically discovers and runs all available workflows
# 2. Port scanning workflow executes (nmap + naabu)
# 3. DNS enumeration workflow runs (nslookup queries)
# 4. Real-time progress shows in terminal
# 5. All results saved to organized log files
# 6. Ready for your HTB writeup!
```

### Output & Logging
- **Automatic execution** - No interaction needed, just run and wait
- **Progress indicators** - See what's running and completed  
- **Organized logs** - Results saved in structured directories
- **Verbose mode** - Add `-v` flag for detailed output
- **Clean output** - Easy to parse for writeups

### Pro Tips for Students
```bash
# See what tools are available
ipcrawler registry list

# Verbose mode (see detailed output)
ipcrawler -v target.com

# Debug mode (see everything that's happening)
ipcrawler --debug target.com

# Check the generated logs after scanning
ls local_files/logs/
```

## 🏗️ How It Works (For the Curious)

**Full transparency:** Most of the complex architecture was implemented with heavy AI assistance since I'm still learning advanced Go programming. But understanding how it works helps me learn!

### The Main Parts (AI helped me design this!)

1. **The Main CLI** (`cmd/ipcrawler/main.go`)
   - Simple command-line interface that just takes a target
   - Automatically discovers and runs all available workflows
   - Shows real-time progress with clean output formatting

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

**Note:** This section is for the development installation only. Production users can skip this!

As a student learning to code, I found these Make commands super helpful for building and testing:

### Development Commands (AI taught me these!)
```bash
# Building and Testing
make build       # Compile Go source into binary
make run         # Test the CLI tool locally  
make dev         # Auto-rebuild on file changes (great for learning!)
make test-all    # Run comprehensive test suite

# Installation and Deployment
make easy        # Create global symlink to your dev build
make install     # Install to $GOPATH/bin (alternative method)
make clean       # Clean build artifacts
```

### Testing Commands (Important for Learning!)
```bash
make test-all      # Complete test suite
make test-static   # Verify CLI architecture correctness
make test-deps     # Check dependency versions
make test-ui       # Test CLI application functionality
```

### Learning by Adding Tools (My Favorite!)
This is how I'm actually learning more about penetration testing:

1. **Study existing patterns** - Look at `tools/nmap/config.yaml`
2. **Create new tool config** - Make `tools/mytool/config.yaml` 
3. **Add to workflows** - Include in `workflows/reconnaissance/`
4. **Test on HTB machines** - Real-world validation!
5. **Iterate and improve** - Learn from what works

**Development tip:** Use `make dev` while coding - it auto-rebuilds when you save files!

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

### Development vs Production

| Method | Use Case | What You Get |
|--------|----------|--------------|
| **Production Install** | HTB practice, normal use | Pre-built binary, automatic setup, just works |
| **Development Install** | Learning, contributing, modifying | Full source, build tools, development environment |

### Development Setup (Detailed)
```bash
git clone https://github.com/neur0map/ipcrawler-go.git
cd ipcrawler-go
make deps        # Install Go dependencies
make build       # Compile from source
make test-all    # Run all tests
make easy        # Create global symlink to your dev build
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
- **PTerm** - For beautiful terminal output management and progress indicators
- **Charmbracelet Log** - For structured logging output
- **Security Community** - For all the awesome tools that IPCrawler integrates
- **Go Ecosystem** - For powerful CLI development capabilities

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

### 🚀 **Quick Start**
**Install production version and start practicing on HTB machines:**
```bash
curl -fsSL https://raw.githubusercontent.com/neur0map/ipcrawler-go/main/install.sh | sudo bash
ipcrawler 10.10.10.10  # Start scanning!
```

### 🛠️ **Developer Start**
**Clone development version for learning and contributing:**
```bash
git clone https://github.com/neur0map/ipcrawler-go.git
cd ipcrawler-go && make easy
ipcrawler 10.10.10.10  # Test your build!
```

---

**Production = Ready to use. Development = Ready to learn and modify.**

*Built by a student, for students. AI-assisted but security-focused. Perfect for Hack The Box practice!* 🚀