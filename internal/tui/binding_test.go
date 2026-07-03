package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Gitlawb/zero/internal/config"
)

func TestParseBindingLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string // expected Label()
	}{
		{"option+o", "Alt+O"},
		{"ctrl+o", "Ctrl+O"},
		{"ctrl+e", "Ctrl+E"},
		{"option+b", "Alt+B"},
		{"option+O", "Alt+O"},
		{"alt+o", "Alt+O"},
		{"cmd+o", "Cmd+O"},
		{"super+o", "Cmd+O"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := parseBinding(tt.input)
			got := p.Label()
			if got != tt.want {
				t.Errorf("parseBinding(%q).Label() = %q, want %q", tt.input, got, tt.want)
			}
			if p.isZero() {
				t.Errorf("parseBinding(%q).isZero() = true, want false", tt.input)
			}
		})
	}
}

func TestOptionOMatchesOptionO(t *testing.T) {
	p := parseBinding("option+o")
	matcher := p.Matcher()

	// Simulate iTerm2 with "Option as Meta" pressing Option+O
	// Terminal sends ESC o → KeyPressEvent{Code: 'o', Mod: ModAlt}
	msg := tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModAlt})

	if !matcher(msg) {
		t.Errorf("option+o matcher should match {Code:'o', Mod:ModAlt}")
	}

	// Should NOT match plain 'o'
	msg2 := tea.KeyPressMsg(tea.Key{Code: 'o'})
	if matcher(msg2) {
		t.Errorf("option+o matcher should NOT match {Code:'o'} without ModAlt")
	}

	// Should NOT match ctrl+o
	msg3 := tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl})
	if matcher(msg3) {
		t.Errorf("option+o matcher should NOT match {Code:'o', Mod:ModCtrl}")
	}
}

func TestOptionOMatchesComposedCharacters(t *testing.T) {
	p := parseBinding("option+o")
	matcher := p.Matcher()

	// On macOS without "Option as Meta", Option+O produces ø (U+00F8)
	msg := tea.KeyPressMsg(tea.Key{Code: 0xF8, Text: "ø"})
	if matcher(msg) {
		t.Logf("NOTE: option+o matcher also matches option+ø (macOS compose behavior)")
	}
}

func TestCtrlEMatchesDefault(t *testing.T) {
	p := parseBinding("ctrl+e")
	matcher := p.Matcher()

	// Terminal sends Ctrl+E → byte 0x05 → KeyPressEvent{Code: 'e', Mod: ModCtrl}
	msg := tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl})

	if !matcher(msg) {
		t.Errorf("ctrl+e matcher should match {Code:'e', Mod:ModCtrl}")
	}
}

func TestConfigToBindingPipeline(t *testing.T) {
	cfg := config.KeyBindingsConfig{
		ToggleDetailed: "option+o",
		ToggleMouse:    "ctrl+e",
		CycleReasoning: "ctrl+t",
		TogglePlan:     "ctrl+p",
		ToggleSidebar:  "option+b",
	}

	bindings := resolveKeyBindings(cfg)

	tests := []struct {
		name    string
		binding parsedBinding
		wantKey string
	}{
		{"toggleDetailed", bindings.toggleDetailed, "Alt+O"},
		{"toggleMouse", bindings.toggleMouse, "Ctrl+E"},
		{"cycleReasoning", bindings.cycleReasoning, "Ctrl+T"},
		{"togglePlan", bindings.togglePlan, "Ctrl+P"},
		{"toggleSidebar", bindings.toggleSidebar, "Alt+B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.binding.isZero() {
				t.Errorf("%s binding is zero (unset) — config not loaded", tt.name)
			}
			if got := tt.binding.Label(); got != tt.wantKey {
				t.Errorf("%s.Label() = %q, want %q", tt.name, got, tt.wantKey)
			}
		})
	}
}

func TestHelpBindingSubstitution(t *testing.T) {
	// Verify that the help overlay correctly shows user-configured bindings
	// by checking that Label() returns non-empty for config-sourced bindings.
	cfg := config.KeyBindingsConfig{
		ToggleDetailed: "option+o",
		ToggleSidebar:  "option+b",
	}

	bindings := resolveKeyBindings(cfg)

	if l := bindings.toggleDetailed.Label(); l != "Alt+O" {
		t.Errorf("toggleDetailed.Label() = %q, want %q", l, "Alt+O")
	}
	if l := bindings.toggleSidebar.Label(); l != "Alt+B" {
		t.Errorf("toggleSidebar.Label() = %q, want %q", l, "Alt+B")
	}
}

// TestDispatchOptionO is an integration-style test: it verifies that pressing
// Option+O dispatches as toggleDetailed when configured.
func TestDispatchOptionO(t *testing.T) {
	cfg := config.KeyBindingsConfig{
		ToggleDetailed: "option+o",
	}
	bindings := resolveKeyBindings(cfg)

	// Simulate Option+O in iTerm2 with "Option as Meta" → ESC o → {Code:'o', Mod:ModAlt}
	msg := tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModAlt})
	defaultFn := func(tea.KeyMsg) bool { return false }

	// This is the dispatch check the model uses
	m := model{keyBindings: bindings}
	if !m.keyMatch(bindings.toggleDetailed, msg, defaultFn) {
		t.Errorf("model.keyMatch should match option+o")
	}

	// Also verify default doesn't match
	msgCtrlO := tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl})
	if m.keyMatch(bindings.toggleDetailed, msgCtrlO, defaultFn) {
		t.Errorf("model.keyMatch should NOT match ctrl+o when option+o is configured")
	}
}
