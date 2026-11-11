package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kovmir/addyui/internal/tui/views"
)

// spyView records the messages it receives so the routing tests can
// observe which view got what, without involving any API client.
type spyView struct {
	id        views.ViewID
	initCalls int
	messages  []tea.Msg
}

func (s *spyView) Init() tea.Cmd { s.initCalls++; return nil }

func (s *spyView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.messages = append(s.messages, msg)
	return s, nil
}

func (s *spyView) View() tea.View { return tea.NewView("") }

// newSpyApp builds an AppModel whose views are spies, so routing can
// be observed directly through m.active, m.history and the spies.
func newSpyApp(active views.ViewID, spies ...*spyView) (AppModel, map[views.ViewID]*spyView) {
	m := AppModel{active: active, views: map[views.ViewID]tea.Model{}}
	byID := map[views.ViewID]*spyView{}
	for _, s := range spies {
		m.views[s.id] = s
		byID[s.id] = s
	}
	return m, byID
}

// keyPress builds the key message a spy receives.
func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func TestAppModelCtrlCQuits(t *testing.T) {
	t.Parallel()

	m, _ := newSpyApp(views.ViewList, &spyView{id: views.ViewList})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c produced no command, want a quit command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("ctrl+c command produced %v, want tea.QuitMsg{}", msg)
	}
}

func TestAppModelDelegatesOtherKeysToActiveView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewList}
	details := &spyView{id: views.ViewDetails}
	m, _ := newSpyApp(views.ViewList, list, details)

	msg := keyPress('x')
	m.Update(msg)

	if len(list.messages) != 1 || list.messages[0] != msg {
		t.Errorf("active view got %v, want the key press", list.messages)
	}
	if len(details.messages) != 0 {
		t.Errorf("inactive view received %v, want nothing", details.messages)
	}
}

func TestAppModelSwitchView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewList}
	details := &spyView{id: views.ViewDetails}
	m, byID := newSpyApp(views.ViewList, list, details)

	payload := keyPress('p') // arbitrary stand-in payload
	model, _ := m.Update(views.SwitchViewMsg{To: views.ViewDetails, With: payload})
	m = model.(AppModel)

	if m.active != views.ViewDetails {
		t.Errorf("active = %d, want %d", m.active, views.ViewDetails)
	}
	if len(m.history) != 1 || m.history[0] != views.ViewList {
		t.Errorf("history = %v, want [ViewList]", m.history)
	}
	if byID[views.ViewDetails].initCalls != 1 {
		t.Errorf("new view Init called %d times, want 1", byID[views.ViewDetails].initCalls)
	}
	got := byID[views.ViewDetails].messages
	if len(got) != 1 || got[0] != payload {
		t.Errorf("new view received %v, want the payload", got)
	}
	if len(list.messages) != 0 {
		t.Errorf("old view received %v, want nothing", list.messages)
	}
}

func TestAppModelSwitchToSameViewIsNoOp(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewList}
	m, byID := newSpyApp(views.ViewList, list)

	model, _ := m.Update(views.SwitchViewMsg{To: views.ViewList})
	m = model.(AppModel)

	if m.active != views.ViewList {
		t.Errorf("active = %d, want %d", m.active, views.ViewList)
	}
	if len(m.history) != 0 {
		t.Errorf("history = %v, want empty: switching to the active view must not push history", m.history)
	}
	if byID[views.ViewList].initCalls != 0 {
		t.Error("Init must not run again for the already active view")
	}
}

func TestAppModelPopView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewList}
	details := &spyView{id: views.ViewDetails}
	m, byID := newSpyApp(views.ViewList, list, details)

	// Push: list -> details.
	model, _ := m.Update(views.SwitchViewMsg{To: views.ViewDetails})
	m = model.(AppModel)

	// Pop: details -> list, with a payload for the restored view.
	payload := keyPress('r')
	model, _ = m.Update(views.PopViewMsg{With: payload})
	m = model.(AppModel)

	if m.active != views.ViewList {
		t.Errorf("active = %d, want %d", m.active, views.ViewList)
	}
	if len(m.history) != 0 {
		t.Errorf("history = %v, want empty after popping", m.history)
	}
	got := byID[views.ViewList].messages
	if len(got) != 1 || got[0] != payload {
		t.Errorf("restored view received %v, want the payload", got)
	}
}

func TestAppModelPopViewWithoutHistoryQuits(t *testing.T) {
	t.Parallel()

	m, _ := newSpyApp(views.ViewList, &spyView{id: views.ViewList})
	_, cmd := m.Update(views.PopViewMsg{})
	if cmd == nil {
		t.Fatal("pop without history produced no command, want a quit command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("pop without history produced %v, want tea.QuitMsg{}", msg)
	}
}

func TestAppModelBroadcastsWindowSize(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewList}
	details := &spyView{id: views.ViewDetails}
	help := &spyView{id: views.ViewHelp}
	m, byID := newSpyApp(views.ViewList, list, details, help)

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	m.Update(msg)

	// Every view must hear the resize, not just the active one, so
	// each view renders correctly when it becomes active later.
	for id, s := range byID {
		if len(s.messages) != 1 || s.messages[0] != msg {
			t.Errorf("view %d received %v, want the window size message", id, s.messages)
		}
	}
}
