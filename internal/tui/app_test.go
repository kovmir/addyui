// SPDX-License-Identifier: MIT

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

	// Commands the spy hands back, so tests can exercise how the root
	// model tags view commands. Both default to nil.
	updateCmd tea.Cmd // Returned by every Update call.
	initCmd   tea.Cmd // Returned by Init.
}

func (s *spyView) Init() tea.Cmd {
	s.initCalls++
	return s.initCmd
}

func (s *spyView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.messages = append(s.messages, msg)
	return s, s.updateCmd
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

	m, _ := newSpyApp(views.ViewAliasList, &spyView{id: views.ViewAliasList})
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

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails}
	m, _ := newSpyApp(views.ViewAliasList, list, details)

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

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails}
	m, byID := newSpyApp(views.ViewAliasList, list, details)

	payload := keyPress('p') // arbitrary stand-in payload
	model, _ := m.Update(views.SwitchViewMsg{To: views.ViewAliasDetails, With: payload})
	m = model.(AppModel)

	if m.active != views.ViewAliasDetails {
		t.Errorf("active = %d, want %d", m.active, views.ViewAliasDetails)
	}
	if len(m.history) != 1 || m.history[0] != views.ViewAliasList {
		t.Errorf("history = %v, want [ViewAliasList]", m.history)
	}
	if byID[views.ViewAliasDetails].initCalls != 1 {
		t.Errorf("new view Init called %d times, want 1", byID[views.ViewAliasDetails].initCalls)
	}
	got := byID[views.ViewAliasDetails].messages
	if len(got) != 1 || got[0] != payload {
		t.Errorf("new view received %v, want the payload", got)
	}
	if len(list.messages) != 0 {
		t.Errorf("old view received %v, want nothing", list.messages)
	}
}

func TestAppModelSwitchToSameViewIsNoOp(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList}
	m, byID := newSpyApp(views.ViewAliasList, list)

	model, _ := m.Update(views.SwitchViewMsg{To: views.ViewAliasList})
	m = model.(AppModel)

	if m.active != views.ViewAliasList {
		t.Errorf("active = %d, want %d", m.active, views.ViewAliasList)
	}
	if len(m.history) != 0 {
		t.Errorf("history = %v, want empty: switching to the active view must not push history", m.history)
	}
	if byID[views.ViewAliasList].initCalls != 0 {
		t.Error("Init must not run again for the already active view")
	}
}

func TestAppModelPopView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails}
	m, byID := newSpyApp(views.ViewAliasList, list, details)

	// Push: list -> details.
	model, _ := m.Update(views.SwitchViewMsg{To: views.ViewAliasDetails})
	m = model.(AppModel)

	// Pop: details -> list, with a payload for the restored view.
	payload := keyPress('r')
	model, _ = m.Update(views.PopViewMsg{With: payload})
	m = model.(AppModel)

	if m.active != views.ViewAliasList {
		t.Errorf("active = %d, want %d", m.active, views.ViewAliasList)
	}
	if len(m.history) != 0 {
		t.Errorf("history = %v, want empty after popping", m.history)
	}
	got := byID[views.ViewAliasList].messages
	if len(got) != 1 || got[0] != payload {
		t.Errorf("restored view received %v, want the payload", got)
	}
}

func TestAppModelPopViewWithoutHistoryQuits(t *testing.T) {
	t.Parallel()

	m, _ := newSpyApp(views.ViewAliasList, &spyView{id: views.ViewAliasList})
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

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails}
	help := &spyView{id: views.ViewHelp}
	m, byID := newSpyApp(views.ViewAliasList, list, details, help)

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

// sentinelMsg is an arbitrary command result the routing tests tag and
// deliver, to observe where commands end up.
type sentinelMsg struct{ n int }

// TestAppModelRoutesCommandResultsToIssuingView covers the core fix: a
// command issued by one view must deliver its result to that view even
// if the user switched to another view while it was in flight.
func TestAppModelRoutesCommandResultsToIssuingView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails}
	m, byID := newSpyApp(views.ViewAliasList, list, details)

	// The list issues a command.
	list.updateCmd = func() tea.Msg { return sentinelMsg{n: 1} }
	model, cmd := m.Update(keyPress('r'))
	if cmd == nil {
		t.Fatal("command from the active view was dropped, want it wrapped")
	}
	tagged, ok := cmd().(views.ViewMsg)
	if !ok {
		t.Fatalf("command produced %T, want views.ViewMsg", cmd())
	}
	if tagged.To != views.ViewAliasList {
		t.Errorf("tagged.To = %d, want %d", tagged.To, views.ViewAliasList)
	}

	// Switch to the details view while the command is in flight.
	model, _ = model.(AppModel).Update(views.SwitchViewMsg{To: views.ViewAliasDetails})
	m = model.(AppModel)

	// The result must reach the list, not the active details view.
	m.Update(tagged)

	got := byID[views.ViewAliasList].messages
	if len(got) != 2 { // key press + command result
		t.Fatalf("list received %d messages, want 2", len(got))
	}
	if got[1] != (sentinelMsg{n: 1}) {
		t.Errorf("list received %v, want the command result", got[1])
	}
	if len(byID[views.ViewAliasDetails].messages) != 0 {
		t.Errorf("details received %v, want nothing", byID[views.ViewAliasDetails].messages)
	}
}

// TestAppModelInitCommandTaggedForActiveView checks that startup
// commands are tagged too: a slow first fetch must not be lost when the
// user switches views right after start.
func TestAppModelInitCommandTaggedForActiveView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList,
		initCmd: func() tea.Msg { return sentinelMsg{n: 1} }}
	m, _ := newSpyApp(views.ViewAliasList, list)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned no command, want the view's init command")
	}
	tagged, ok := cmd().(views.ViewMsg)
	if !ok {
		t.Fatalf("Init command produced %T, want views.ViewMsg", cmd())
	}
	if tagged.To != views.ViewAliasList || tagged.Msg != (sentinelMsg{n: 1}) {
		t.Errorf("tagged = %+v, want the init command tagged for the list", tagged)
	}
}

// TestAppModelSwitchViewTagsInitForNewView checks that the Init command
// run on a view switch is tagged for the view switched to.
func TestAppModelSwitchViewTagsInitForNewView(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails,
		initCmd: func() tea.Msg { return sentinelMsg{n: 2} }}
	m, _ := newSpyApp(views.ViewAliasList, list, details)

	// No payload: the returned batch reduces to the Init command alone.
	_, cmd := m.Update(views.SwitchViewMsg{To: views.ViewAliasDetails})
	tagged, ok := cmd().(views.ViewMsg)
	if !ok {
		t.Fatalf("command produced %T, want views.ViewMsg", cmd())
	}
	if tagged.To != views.ViewAliasDetails || tagged.Msg != (sentinelMsg{n: 2}) {
		t.Errorf("tagged = %+v, want the init command tagged for details", tagged)
	}
}

// TestAppModelRootCommandsPassThroughTagging checks that navigation and
// quit commands returned by views are not wrapped, so the root model
// keeps handling them.
func TestAppModelRootCommandsPassThroughTagging(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList}
	details := &spyView{id: views.ViewAliasDetails}
	m, _ := newSpyApp(views.ViewAliasList, list, details)

	list.updateCmd = views.SwitchView(views.ViewAliasDetails)
	_, cmd := m.Update(keyPress('f'))
	if msg := cmd(); msg != (views.SwitchViewMsg{To: views.ViewAliasDetails}) {
		t.Fatalf("switch command produced %#v, want it unwrapped", msg)
	}

	list.updateCmd = tea.Quit
	_, cmd = m.Update(keyPress('q'))
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("quit command produced %#v, want tea.QuitMsg", msg)
	}
}

// TestAppModelBatchCommandsStayTagged checks that commands a view
// batches keep their tag (the create view batches two fetches in Init).
func TestAppModelBatchCommandsStayTagged(t *testing.T) {
	t.Parallel()

	list := &spyView{id: views.ViewAliasList}
	m, _ := newSpyApp(views.ViewAliasList, list)

	list.updateCmd = tea.Batch(
		func() tea.Msg { return sentinelMsg{n: 1} },
		func() tea.Msg { return sentinelMsg{n: 2} },
	)
	_, cmd := m.Update(keyPress('r'))

	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("command produced %T, want tea.BatchMsg", cmd())
	}
	if len(batch) != 2 {
		t.Fatalf("batch holds %d commands, want 2", len(batch))
	}
	for i, c := range batch {
		tagged, ok := c().(views.ViewMsg)
		if !ok {
			t.Fatalf("batch command %d produced %T, want views.ViewMsg", i, c())
		}
		if tagged.To != views.ViewAliasList {
			t.Errorf("batch command %d tagged for %d, want %d",
				i, tagged.To, views.ViewAliasList)
		}
	}
}
