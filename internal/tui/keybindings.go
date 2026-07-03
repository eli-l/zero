package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Gitlawb/zero/internal/config"
)

// parsedBinding is the parsed representation of a keybinding string such as
// "ctrl+o" or "alt+enter". The zero value is a sentinel meaning "use default".
type parsedBinding struct {
	ctrl  bool
	alt   bool
	shift bool
	cmd   bool   // ⌘ Command (macOS) / ⊞ Win → ModSuper
	code  rune   // tea.KeyCode or 0 for text-based matching
	text  string // for text-based matching (e.g. "?")
}

// isZero returns true when p is the nil sentinel (no binding configured).
func (p parsedBinding) isZero() bool {
	return p.code == 0 && p.text == ""
}

// Label returns a human-readable representation of the binding, e.g. "Ctrl+O"
// or "Cmd+Shift+Enter". Used in the help overlay. Returns empty string for
// zero (unset) bindings.
func (p parsedBinding) Label() string {
	if p.isZero() {
		return ""
	}
	var b strings.Builder
	if p.ctrl {
		b.WriteString("Ctrl+")
	}
	if p.alt {
		b.WriteString("Alt+")
	}
	if p.shift {
		b.WriteString("Shift+")
	}
	if p.cmd {
		b.WriteString("Cmd+")
	}
	if p.text != "" {
		b.WriteString(p.text)
	} else if p.code != 0 {
		switch p.code {
		case tea.KeyEnter:
			b.WriteString("Enter")
		case tea.KeyTab:
			b.WriteString("Tab")
		case tea.KeyEsc:
			b.WriteString("Esc")
		case tea.KeySpace:
			b.WriteString("Space")
		case tea.KeyBackspace:
			b.WriteString("Backspace")
		case tea.KeyDelete:
			b.WriteString("Delete")
		case tea.KeyUp:
			b.WriteString("↑")
		case tea.KeyDown:
			b.WriteString("↓")
		case tea.KeyLeft:
			b.WriteString("←")
		case tea.KeyRight:
			b.WriteString("→")
		case tea.KeyHome:
			b.WriteString("Home")
		case tea.KeyEnd:
			b.WriteString("End")
		case tea.KeyPgUp:
			b.WriteString("PgUp")
		case tea.KeyPgDown:
			b.WriteString("PgDn")
		default:
			// Printable character — uppercase for display
			if p.code >= 'a' && p.code <= 'z' {
				b.WriteRune(p.code - 32)
			} else {
				b.WriteRune(p.code)
			}
		}
	}
	return b.String()
}

// Matcher returns a function that matches a tea.KeyMsg against this binding.
// It is the hot path for the key dispatch — kept cheap intentionally.
func (p parsedBinding) Matcher() func(tea.KeyMsg) bool {
	if p.isZero() {
		// A zero binding should never be matched — the caller is expected to
		// check useDefault() first and fall through to the built-in check.
		return func(tea.KeyMsg) bool { return false }
	}

	// Build the required modifier mask from the parsed flags.
	var mod tea.KeyMod
	if p.ctrl {
		mod |= tea.ModCtrl
	}
	if p.alt {
		mod |= tea.ModAlt
	}
	if p.shift {
		mod |= tea.ModShift
	}
	if p.cmd {
		mod |= tea.ModSuper // ⌘ Command on macOS, ⊞ Win on Windows
	}

	// ctrl+letter fast path — the terminal may report this as a control
	// character code (e.g. ctrl+o → 0x0F), so use keyCtrl which handles
	// both the code and ModCtrl representations.
	if mod == tea.ModCtrl && p.code >= 'a' && p.code <= 'z' {
		return func(msg tea.KeyMsg) bool {
			return keyCtrl(msg, p.code)
		}
	}

	if p.text != "" {
		return func(msg tea.KeyMsg) bool {
			return msg.Key().Text == p.text && msg.Key().Mod.Contains(mod)
		}
	}

	return func(msg tea.KeyMsg) bool {
		return msg.Key().Code == p.code && msg.Key().Mod.Contains(mod)
	}
}

// parseBinding converts a user-written keybinding string (e.g. "ctrl+o") into
// a parsedBinding. The empty string returns zero parsedBinding (the "use
// default" sentinel).
func parseBinding(s string) parsedBinding {
	s = strings.TrimSpace(s)
	if s == "" {
		return parsedBinding{}
	}

	parts := strings.Split(strings.ToLower(s), "+")
	var p parsedBinding
	var keyPart string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl", "control":
			p.ctrl = true
		case "alt", "option":
			p.alt = true
		case "shift":
			p.shift = true
		case "cmd", "command":
			p.cmd = true
		case "super":
			// "super" matches the Bubble Tea naming; on macOS this is ⌘
			p.cmd = true
		default:
			keyPart = part
		}
	}

	if p.ctrl && len(keyPart) == 1 && keyPart[0] >= 'a' && keyPart[0] <= 'z' {
		// ctrl+<letter> — store the code so the match is exact
		p.code = []rune(keyPart)[0]
		p.text = ""
		return p
	}

	// Map named keys to their tea.KeyCode
	switch keyPart {
	case "enter", "return":
		p.code = tea.KeyEnter
	case "tab":
		p.code = tea.KeyTab
	case "esc", "escape":
		p.code = tea.KeyEsc
	case "space":
		p.code = tea.KeySpace
	case "backspace":
		p.code = tea.KeyBackspace
	case "delete":
		p.code = tea.KeyDelete
	case "up":
		p.code = tea.KeyUp
	case "down":
		p.code = tea.KeyDown
	case "left":
		p.code = tea.KeyLeft
	case "right":
		p.code = tea.KeyRight
	case "home":
		p.code = tea.KeyHome
	case "end":
		p.code = tea.KeyEnd
	case "pgup", "pageup":
		p.code = tea.KeyPgUp
	case "pgdown", "pagedown":
		p.code = tea.KeyPgDown
	case "?":
		p.text = "?"
		p.code = 0
	default:
		// Single character, any modifier context
		if len(keyPart) == 1 {
			p.code = []rune(keyPart)[0]
		}
		// else unrecognised — leave zero so it falls through to default
	}

	return p
}

// labelOr returns b.Label() when b is configured (non-zero), otherwise it
// returns the caller-supplied default label string.  This is the display-layer
// counterpart to keyMatch — dispatch falls through to the hardcoded default
// function, so the label displayed in help / hints must match that fallback.
func labelOr(b parsedBinding, defaultLabel string) string {
	if !b.isZero() {
		return b.Label()
	}
	return defaultLabel
}

// keyBindings holds the parsed, resolved key bindings the model uses at
// dispatch time. When a binding's parsedBinding is zero, the built-in default
// check in model.go's updateModel should be used.
type keyBindings struct {
	toggleDetailed parsedBinding
	toggleMouse    parsedBinding
	cycleReasoning parsedBinding
	togglePlan     parsedBinding
	toggleSidebar  parsedBinding
}

// resolveKeyBindings converts a user-facing KeyBindingsConfig into the
// dispatch-ready parsed form, using empty-is-default semantics.
func resolveKeyBindings(cfg config.KeyBindingsConfig) keyBindings {
	return keyBindings{
		toggleDetailed: parseBinding(string(cfg.ToggleDetailed)),
		toggleMouse:    parseBinding(string(cfg.ToggleMouse)),
		cycleReasoning: parseBinding(string(cfg.CycleReasoning)),
		togglePlan:     parseBinding(string(cfg.TogglePlan)),
		toggleSidebar:  parseBinding(string(cfg.ToggleSidebar)),
	}
}

// builtinBindings holds the default hardcoded bindings. keybinding config
// overrides the built-in on a per-action basis when the field is non-empty.
var builtinBindings = keyBindings{
	// All zero: the model.go dispatch uses hardcoded key-case comparisons.
	// These are the canonical defaults and are never matched through the
	// parsedBinding.Matcher path — the config-override path is separate.
}

// keyMatch returns true when msg matches either the user-configured binding b
// or (when b is zero/unset) the built-in default matcher defaultFn. This is
// the bridge between the config surface and the hot dispatch path in model.go.
func (m model) keyMatch(b parsedBinding, msg tea.KeyMsg, defaultFn func(tea.KeyMsg) bool) bool {
	if !b.isZero() {
		return b.Matcher()(msg)
	}
	return defaultFn(msg)
}
