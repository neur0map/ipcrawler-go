Objective: Rebuild the frontend terminal TUI from scratch using Charmbracelet only — Bubble Tea (runtime), Bubbles (components), Lipgloss (styling/layout), Glamour (markdown help), Log+termenv (logging/colors). Scope is STRICTLY UI: no backend/tool execution/CLIs/data schemas. Deliver a modern, minimalistic (grayscale + subtle accents), fully interactive, resize-safe UI with zero overlap and zero flicker. Backend starts only after this TUI is approved.

Research-first (must complete before coding):
- Use Context7 MCP + web search to gather: Bubble Tea MVU & WindowSizeMsg; Bubbles (list/table/viewport/progress/spinner/textarea/key); Lipgloss borders/joins/padding/margins/wrap; Glamour; Log/termenv; responsive TUI layout patterns; alt-screen/viewport performance tips.
- Produce a 1–2 page brief: Tea vs Bubbles roles; initial-render vs WindowSizeMsg gating; responsive layout rules; color/contrast policy; citations.

Hard constraints (tests, not suggestions):
1) TUI-only edits: /internal/ui/**, /internal/term/**, /configs/ui.yaml, and minimal glue in cmd/<cli>/main.go. Nothing else.
2) Single renderer: exactly one tea.Program + one render loop. No duplicate TUIs or ad-hoc redraw hacks.
3) No magic numbers: sizes/strings/icons come from /configs/ui.yaml and /internal/ui/theme tokens.
4) Resize-safe: handle WindowSizeMsg; compute widths/heights via Lipgloss; no overlap; no truncating critical content; adaptive at S/M/L widths.
5) Stable line count: in-place updates do not add lines (no scroll creep/flicker).
6) Interaction: arrows navigate; Space toggles/selects; Enter confirms; Tab cycles focus; q quits; ? opens help; viewport scroll works.
7) Visuals: minimal monochrome with subtle accent; respect termenv color profile; non-TTY fallback prints clean logs (no ANSI).

Architecture & implementation:
- Bubble Tea for state/update/view; Bubbles for list/table/viewport/progress/spinner/help; Lipgloss for boxes/layout; Glamour for help.
- Initial-render guard: do not compute layout until first WindowSizeMsg; store dims; set ready=true.
- Layout breakpoints (from /configs/ui.yaml):
  • Large: Left Nav | Main Content | Right Status
  • Medium: Left Nav | Main Content (status footer)
  • Small: stacked screens with key/tabs to switch
  • Compose using Lipgloss JoinHorizontal/JoinVertical only.
- Panels:
  • Left: workflows/tools (list or table) with focus + Space to select/toggle.
  • Center: details + streaming logs (viewport, high-perf; sticky header).
  • Right/footer: live status (spinner/progress, counts, errors).
- Status line must update in place: “Starting workflows: (1/5) running” → completion without adding lines.
- For now, feed content from a simulator behind an interface (no backend changes).

Testing & acceptance (CI required):
- Make targets: deps, build, demo, test-ui.
  deps: pin latest majors: bubbletea, bubbles, lipgloss, glamour, log, termenv.
  demo: run simulator in alt-screen to showcase layout + interactions.
  test-ui (golden frames):
    a) NO line growth across N updates,
    b) NO overlap at 80x24, 100x30, 120x40, 160x48,
    c) keys (arrows/space/enter/tab/q/?) work,
    d) non-TTY output has zero ANSI,
    e) static check: exactly one tea.NewProgram in repo.
- Perf sanity: responsive under 1k events/min + rapid resizes; cap animation FPS via tea.Every if needed.

Deliverables:
- PR with code/tests/configs/README (“make deps && make demo”) + asciinema.
- Design note (Tea vs Bubbles; refresh strategy; responsive rules; citations).
- Troubleshooting: common overlap/line-growth causes and our mitigations.

Exit criteria:
- Frontend TUI works end-to-end with Charmbracelet only; no overlaps; no duplicate TUIs; no hardcoded sizes; interactions solid; status line updates in place; CI green.
