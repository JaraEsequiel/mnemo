package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	hintStyle   = lipgloss.NewStyle().Faint(true)
	choiceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
)

type labeledChoice struct{ name, desc string }

// targetChoices is the (extensible) list of install platforms. Add a new target
// here, give it a branch in buildResults, and it shows up as a checkbox.
var targetChoices = []labeledChoice{
	{"Local", "this machine (~/.claude, MCP user-scope)"},
	{"Cowork", "project-scoped config in the folder (cloud/sandbox)"},
}

const (
	targetLocal = iota
	targetCowork
)

// vaultSubdir is the memory vault's location relative to the chosen base folder.
const vaultSubdir = "Memory"

var scopeChoices = []labeledChoice{
	{"user", "all your projects (recommended)"},
	{"local", "only the current project (private)"},
	{"project", "shared via .mcp.json in the project"},
}

// wizard steps.
const (
	stepTargets = iota
	stepBase
	stepLocalScope
	stepConfirm
)

type wizard struct {
	step      int
	cursor    int          // cursor in the targets multi-select
	selected  map[int]bool // which targets are checked
	ti        textinput.Model
	scopeIdx  int
	graph     bool
	base      string // the one folder the user marks
	baseDef   string
	opts      Options
	results   []Options
	confirmed bool
}

func newWizard(def Options) wizard {
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 50
	// Default base folder: an explicit --target/--vault dir, else the home dir.
	baseDef := def.Target
	if baseDef == "" && def.Vault != "" {
		baseDef = strings.TrimSuffix(def.Vault, string(os.PathSeparator)+vaultSubdir)
	}
	if baseDef == "" {
		baseDef, _ = os.UserHomeDir()
	}
	return wizard{
		ti:       ti,
		opts:     def,
		graph:    true,
		selected: map[int]bool{targetLocal: true}, // Local pre-checked
		baseDef:  baseDef,
	}
}

func (w wizard) Init() tea.Cmd { return textinput.Blink }

func (w wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		w.ti, cmd = w.ti.Update(msg)
		return w, cmd
	}
	switch key.String() {
	case "ctrl+c", "esc":
		return w, tea.Quit
	case "enter":
		return w.advance()
	}

	switch w.step {
	case stepTargets:
		switch key.String() {
		case "up", "k":
			if w.cursor > 0 {
				w.cursor--
			}
		case "down", "j":
			if w.cursor < len(targetChoices)-1 {
				w.cursor++
			}
		case " ", "x":
			w.selected[w.cursor] = !w.selected[w.cursor]
		}
	case stepBase:
		var cmd tea.Cmd
		w.ti, cmd = w.ti.Update(msg)
		return w, cmd
	case stepLocalScope:
		switch key.String() {
		case "up", "k":
			if w.scopeIdx > 0 {
				w.scopeIdx--
			}
		case "down", "j":
			if w.scopeIdx < len(scopeChoices)-1 {
				w.scopeIdx++
			}
		}
	case stepConfirm:
		if w.selected[targetLocal] && key.String() == "g" {
			w.graph = !w.graph
		}
	}
	return w, nil
}

func (w wizard) advance() (tea.Model, tea.Cmd) {
	switch w.step {
	case stepTargets:
		if !w.selected[targetLocal] && !w.selected[targetCowork] {
			return w, nil // require at least one
		}
		w.ti.SetValue(w.baseDef)
		w.ti.Focus()
		w.step = stepBase
	case stepBase:
		if strings.TrimSpace(w.ti.Value()) == "" {
			return w, nil
		}
		w.base = strings.TrimSpace(w.ti.Value())
		if w.selected[targetLocal] {
			w.step = stepLocalScope
		} else {
			w.step = stepConfirm
		}
	case stepLocalScope:
		w.step = stepConfirm
	case stepConfirm:
		w.confirmed = true
		w.results = w.buildResults()
		return w, tea.Quit
	}
	return w, nil
}

// vaultPath returns the memory vault location: <base>/Memory.
func (w wizard) vaultPath() string { return filepath.Join(w.base, vaultSubdir) }

func (w wizard) buildResults() []Options {
	var out []Options
	if w.selected[targetLocal] {
		o := w.opts
		o.Vault = w.vaultPath()
		o.Scope = scopeChoices[w.scopeIdx].name
		o.WriteGraph = w.graph
		o.RegisterMCP = true
		o.Cowork = false
		o.Target = ""
		out = append(out, o)
	}
	if w.selected[targetCowork] {
		o := w.opts
		o.Cowork = true
		o.Target = w.base // RunCowork puts the vault at <base>/Memory itself
		out = append(out, o)
	}
	return out
}

func (w wizard) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mnemo setup") + "\n\n")
	switch w.step {
	case stepTargets:
		b.WriteString("Install for which platform(s)?  (space to toggle, one or more)\n\n")
		for i, c := range targetChoices {
			cursor := "  "
			if i == w.cursor {
				cursor = cursorStyle.Render("▸ ")
			}
			box := "[ ]"
			if w.selected[i] {
				box = "[x]"
			}
			line := fmt.Sprintf("%s %s — %s", box, c.name, c.desc)
			if w.selected[i] {
				line = choiceStyle.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n" + hintStyle.Render("↑/↓ move · space toggle · enter continue · esc cancel"))
	case stepBase:
		b.WriteString("Folder to install into:\n")
		b.WriteString(w.ti.View() + "\n\n")
		base := strings.TrimSpace(w.ti.Value())
		b.WriteString(hintStyle.Render(fmt.Sprintf("vault → %s/%s", base, vaultSubdir)))
		if w.selected[targetCowork] {
			b.WriteString(hintStyle.Render(fmt.Sprintf("  ·  Cowork config → %s/", base)))
		}
		b.WriteString("\n" + hintStyle.Render("enter to continue · esc to cancel"))
	case stepLocalScope:
		b.WriteString("Local — MCP server scope:\n\n")
		renderChoices(&b, scopeChoices, w.scopeIdx)
		b.WriteString("\n" + hintStyle.Render("↑/↓ to choose · enter to continue"))
	case stepConfirm:
		b.WriteString("Ready to install:\n\n")
		fmt.Fprintf(&b, "  folder:  %s\n", w.base)
		fmt.Fprintf(&b, "  vault:   %s\n", w.vaultPath())
		if w.selected[targetLocal] {
			graph := "yes"
			if !w.graph {
				graph = "no"
			}
			fmt.Fprintf(&b, "  Local    scope=%s · graph=%s %s\n",
				scopeChoices[w.scopeIdx].name, graph, hintStyle.Render("(g toggles graph)"))
		}
		if w.selected[targetCowork] {
			fmt.Fprintf(&b, "  Cowork   project-scoped config in the folder\n")
		}
		b.WriteString("\n" + hintStyle.Render("enter to install · esc to cancel"))
	}
	b.WriteString("\n")
	return b.String()
}

func renderChoices(b *strings.Builder, choices []labeledChoice, idx int) {
	for i, c := range choices {
		cursor := "  "
		line := fmt.Sprintf("%s — %s", c.name, c.desc)
		if i == idx {
			cursor = cursorStyle.Render("▸ ")
			line = choiceStyle.Render(line)
		}
		b.WriteString(cursor + line + "\n")
	}
}

// RunWizard launches the interactive setup TUI. The user marks one base folder;
// the vault is <base>/Memory and (if chosen) Cowork config goes in <base>.
// Returns one Options per selected platform, or an error if cancelled.
func RunWizard(def Options) ([]Options, error) {
	m, err := tea.NewProgram(newWizard(def)).Run()
	if err != nil {
		return nil, err
	}
	w, _ := m.(wizard)
	if !w.confirmed || len(w.results) == 0 {
		return nil, fmt.Errorf("setup cancelled")
	}
	return w.results, nil
}
