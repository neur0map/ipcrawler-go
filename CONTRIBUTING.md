# Contributing: Add New Tools and Workflows (No Coding Required)

This project is designed for cybersecurity enthusiasts. You do not need to write code to add new tools or workflows. Everything is done with simple folders and YAML files.

If you can copy/paste and edit a few lines, you can contribute.

---

## What You’ll Do

- Add a new external tool (like `nmap`, `naabu`, `ffuf`) by creating a small config file.
- Create a workflow (a recipe of steps) that uses one or more tools.
- Run the workflow against a target like `example.com` and view results in `out/` and reports in `reports/`.

---

## Before You Start

1) Build the IPCrawler binary (one-time):

```bash
make build
```

This produces `./build/ipcrawler`.

2) Install any external tools you want to use (on macOS, examples):

```bash
brew install nmap
brew install projectdiscovery/tap/naabu
brew install ffuf
brew install bind  # for dig
```

Tip: You can check installed tools with `which <toolname>` (e.g., `which nmap`).

3) Confirm IPCrawler runs:

```bash
./build/ipcrawler list
```

---

## Glossary (Simple)

- Tool: An external program you run, like `nmap` or `naabu`.
- Workflow: A recipe describing which tools to run, in what order, and where to save outputs.
- Target: What you are scanning (e.g., `example.com`, an IP, or a file with targets).

---

## Add a New Tool (5–10 minutes)

You’ll create a folder and a single `config.yaml`. IPCrawler auto-detects it — no code changes.

1) Create the tool folder:

```
tools/<your_tool_name>/
```

2) Create `tools/<your_tool_name>/config.yaml` using this minimal template:

```yaml
name: <your_tool_name>          # must match folder name
command: <your_tool_command>    # how you run it in your terminal (e.g., nmap)
output: json                    # json | xml | text (pick what the tool outputs best)
args:
  default: []                   # arguments always used (start empty if unsure)
  flags:                        # named presets you can reuse in workflows
    basic: []                   # add more later (fast, full, etc.)
mappings: []                    # optional (advanced); you can leave this empty
```

Examples (existing tools):

- `naabu` (JSON output): see `tools/naabu/config.yaml`
- `nmap` (XML output): see `tools/nmap/config.yaml`
- `dig` / `nslookup` (text output): see `tools/dig/config.yaml`, `tools/nslookup/config.yaml`

3) Optional (nice to have):

- `tools/<your_tool_name>/docs.md` — notes for users (what the flags mean)
- `tools/<your_tool_name>/example_output.json|xml|txt` — sample raw output

4) Verify that IPCrawler detects your tool:

```bash
./build/ipcrawler list
# Look for your tool name under “Available tools”
```

If it’s missing, check:

- The folder path is exactly `tools/<name>/config.yaml`
- `name:` in the YAML matches the folder name
- The `command:` you set exists on your system (e.g., `which <command>`)

---

## Create a Workflow (2–5 minutes)

Workflows are YAML files that describe steps. Each step runs a tool or performs a simple built-in action. You can run multiple workflows together.

1) Pick a folder (you can reuse existing categories or make your own):

```
workflows/basic/
workflows/dns/
custom-workflows/
```

2) Create a file like `custom-workflows/my_first_workflow.yaml` using this template:

```yaml
id: my_first_workflow
description: Run a quick scan using my tool
parallel: true  # can run in parallel with other workflows

steps:
  - id: run_my_tool
    tool: <your_tool_name>
    use_flags: basic          # the flag set name from your tool config
    override_args:            # extra arguments specific to this step
      - "{{target}}"          # this inserts the target at runtime
    output: "out/{{target}}/<your_tool_name>.out"
```

Notes:

- `{{target}}` is automatically replaced with what you pass on the command line (e.g., `example.com`).
- `output:` is where the tool’s output is saved. Keep it under `out/{{target}}/`.
- `use_flags:` refers to a named flag set in your tool’s `config.yaml`.

3) Run your workflow:

```bash
./build/ipcrawler example.com --workflow my_first_workflow
```

Check results in:

- Raw outputs: `out/example.com/`
- Reports: `reports/example.com/` (generated after workflows complete)

---

## Example: Two-Step Workflow (Scan then Fingerprint)

```yaml
id: quick_portscan_and_fingerprint
description: Fast port scan with naabu, then fingerprint with nmap
parallel: true

steps:
  - id: scan_fast
    tool: naabu
    use_flags: fast
    override_args:
      - "{{target}}"
    output: "out/{{target}}/naabu_fast.json"

  - id: fingerprint
    tool: nmap
    use_flags: fingerprint
    override_args:
      - "-iL"
      - "out/{{target}}/hostlist.txt"  # assume you generated this in a previous step
      - "-oX"
      - "out/{{target}}/nmap_fingerprint.xml"
    output: "out/{{target}}/nmap_fingerprint.xml"
    depends_on: [scan_fast]
```

Tip: See working examples in `workflows/basic/` and `workflows/dns/`.

---

## Troubleshooting (Quick Fixes)

- Tool not found: install it (e.g., `brew install nmap`) and confirm with `which nmap`.
- No output file: check the `output:` path in your workflow; folders are created automatically under `out/{{target}}/`.
- Wrong flags: adjust `use_flags:` in the workflow or update the tool’s `config.yaml`.
- Permission issues: some tools (like certain `nmap` scans) may need elevated privileges.

---

## Safety & Etiquette

- Only scan targets you have permission to test.
- Avoid aggressive modes unless you know what you’re doing.
- Keep outputs inside `out/` and `reports/` (default behavior).

---

## Optional (Advanced) – Output Mappings

You can teach IPCrawler how to extract structured fields from JSON/XML outputs with `mappings`, but this is optional. If you skip it, raw tool outputs are still saved to files.

Basic idea (JSON):

```yaml
mappings:
  - type: port
    query: "[]"          # treat the root JSON as a list
    fields:
      ip: "ip"
      port: "port"
      protocol: "proto"
```

If this looks unfamiliar, ignore it for now. Your contributions are still useful without mappings.

---

## How to Share Your Contribution

1) Test locally: `./build/ipcrawler example.com --workflow <your_workflow_id>`
2) Commit and push your changes:

```bash
git add tools/<your_tool_name>/ workflows/**/<your_workflow>.yaml
git commit -m "Add <your_tool_name> and workflow <your_workflow_id>"
git push
```

That’s it! Your tool and workflows are configuration-only and should be easy for others to use.

---

## Need Inspiration?

- Look at: `tools/naabu/config.yaml`, `tools/nmap/config.yaml`, `workflows/basic/`.
- Use `./build/ipcrawler list` to see available tools and workflows.

