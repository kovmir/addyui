package views

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

const helpText = `ALIAS LIST     enter details · c create · space toggle active · p pin · d delete
               x forget (twice) · f pin filter · v state filter · / search
               r refetch · h/l or arrows page · esc back · q quit
ALIAS DETAILS  j/k interactive row · J/K scroll · space act on row · p/n prev/next alias · esc back · q quit
NEW ALIAS      space edit field · enter submit · esc back · q quit`

type HelpModel struct {
	vp viewport.Model
}

func NewHelpModel() HelpModel {
	vp := viewport.New()
	vp.SetContent(helpText)
	return HelpModel{vp: vp}
}

func (m HelpModel) Init() tea.Cmd { return nil }

func (m HelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.vp.SetWidth(msg.Width)
		m.vp.SetHeight(msg.Height - 1)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "f1", "esc":
			return m, PopView()
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m HelpModel) View() tea.View {
	var b strings.Builder
	b.WriteString(m.vp.View())
	b.WriteString("\npress F1 to return")
	return tea.NewView(b.String())
}
