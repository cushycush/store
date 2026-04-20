package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cushycush/store/internal/secrets"
)

type secretsPhase int

const (
	secretsAskPass secretsPhase = iota
	secretsList
)

// Secrets is the overlay for managing encrypted secrets.
type Secrets struct {
	root       string
	passphrase string
	phase      secretsPhase

	pass textinput.Model

	values map[string]string
	names  []string
	cursor int
	reveal map[string]bool

	sub       *Input
	subAction string // "add_name" | "add_value" | "edit" | "confirm_delete"
	subCtx    string

	err  string
	done bool
}

// NewSecrets constructs the overlay. If STORE_PASSPHRASE is set we skip
// the prompt phase and load immediately.
func NewSecrets(root string) *Secrets {
	o := &Secrets{root: root, reveal: make(map[string]bool)}
	ti := textinput.New()
	ti.EchoMode = textinput.EchoPassword
	ti.Prompt = ""
	ti.Focus()
	o.pass = ti
	return o
}

// Update processes a key. done=true means close the overlay.
func (o *Secrets) Update(msg tea.Msg) tea.Cmd {
	if o.sub != nil {
		cmd := o.sub.Update(msg)
		if o.sub.Done() {
			if !o.sub.Cancelled() {
				o.applySub(o.sub.Value())
			}
			o.sub = nil
		}
		return cmd
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if k.String() == "esc" {
		o.done = true
		return nil
	}
	switch o.phase {
	case secretsAskPass:
		if k.String() == "enter" {
			o.passphrase = o.pass.Value()
			if err := o.load(); err != nil {
				o.err = err.Error()
			} else {
				o.phase = secretsList
				o.err = ""
			}
			return nil
		}
		var cmd tea.Cmd
		o.pass, cmd = o.pass.Update(msg)
		return cmd
	case secretsList:
		return o.updateList(k)
	}
	return nil
}

func (o *Secrets) updateList(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "j", "down":
		if o.cursor < len(o.names)-1 {
			o.cursor++
		}
	case "k", "up":
		if o.cursor > 0 {
			o.cursor--
		}
	case "v":
		if len(o.names) > 0 {
			n := o.names[o.cursor]
			o.reveal[n] = !o.reveal[n]
		}
	case "a":
		o.sub = NewInput("new secret name", "stored encrypted; referenced as {{ secret \"name\" }}", "name", false)
		o.subAction = "add_name"
	case "e", "enter":
		if len(o.names) > 0 {
			o.subCtx = o.names[o.cursor]
			o.sub = NewInput("new value for "+o.subCtx, "input hidden; enter commits", "value", true)
			o.subAction = "edit"
		}
	case "d":
		if len(o.names) > 0 {
			o.subCtx = o.names[o.cursor]
			o.sub = NewInput("delete secret "+o.subCtx+"?", "type the secret's name to confirm", o.subCtx, false)
			o.subAction = "confirm_delete"
		}
	}
	return nil
}

func (o *Secrets) applySub(value string) {
	switch o.subAction {
	case "add_name":
		o.subCtx = value
		o.sub = NewInput("value for "+value, "input hidden; enter commits", "value", true)
		o.subAction = "add_value"
	case "add_value":
		if o.values == nil {
			o.values = map[string]string{}
		}
		o.values[o.subCtx] = value
		_ = o.save()
		o.subCtx = ""
		o.subAction = ""
	case "edit":
		if o.values == nil {
			o.values = map[string]string{}
		}
		o.values[o.subCtx] = value
		_ = o.save()
		o.subCtx = ""
		o.subAction = ""
	case "confirm_delete":
		if value == o.subCtx {
			delete(o.values, o.subCtx)
			_ = o.save()
		}
		o.subCtx = ""
		o.subAction = ""
	}
}

func (o *Secrets) load() error {
	m, err := secrets.Load(o.root, o.passphrase)
	if err != nil {
		return err
	}
	o.values = m
	o.rebuildNames()
	return nil
}

func (o *Secrets) save() error {
	if err := secrets.Save(o.root, o.passphrase, o.values); err != nil {
		o.err = err.Error()
		return err
	}
	o.rebuildNames()
	if o.cursor >= len(o.names) && o.cursor > 0 {
		o.cursor--
	}
	return nil
}

func (o *Secrets) rebuildNames() {
	o.names = o.names[:0]
	for name := range o.values {
		o.names = append(o.names, name)
	}
	sort.Strings(o.names)
}

// Done reports whether the overlay should close.
func (o *Secrets) Done() bool { return o.done }

// View renders the overlay body.
func (o *Secrets) View() string {
	if o.sub != nil {
		return o.sub.View()
	}
	var b strings.Builder
	switch o.phase {
	case secretsAskPass:
		b.WriteString(StyleBold.Render("unlock secrets"))
		b.WriteString("\n")
		b.WriteString(StyleDim.Render("passphrase is used to decrypt .store/secrets.enc"))
		b.WriteString("\n\n  ")
		b.WriteString(o.pass.View())
		if o.err != "" {
			b.WriteString("\n\n  ")
			b.WriteString(lipglossError(o.err))
		}
	case secretsList:
		if len(o.names) == 0 {
			b.WriteString(StyleDim.Render("no secrets yet · [a] to add"))
			return b.String()
		}
		for i, n := range o.names {
			marker := "  "
			style := StyleFg
			if i == o.cursor {
				marker = StyleEmber.Render(GlyphCursor) + " "
				style = StyleSelected
			}
			var value string
			if o.reveal[n] {
				value = StyleFg.Render(o.values[n])
			} else {
				value = StyleDim.Render("· · · · · ·")
			}
			b.WriteString(marker + style.Render(padName(n, 20)) + "  " + value + "\n")
		}
	}
	return b.String()
}

// Footer returns the key hint line for the overlay frame.
func (o *Secrets) Footer() string {
	if o.sub != nil {
		return o.sub.Footer()
	}
	switch o.phase {
	case secretsAskPass:
		return StyleDim.Render("enter unlock · esc cancel")
	case secretsList:
		return StyleDim.Render("a add · e edit · d delete · v reveal · esc close")
	}
	return ""
}

func lipglossError(s string) string {
	return StyleDim.Render("err: ") + StyleFg.Render(s)
}
