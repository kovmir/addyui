// SPDX-License-Identifier: MIT

package views

import tea "charm.land/bubbletea/v2"

// ViewID identifies a screen/view in the app. Defined here rather than in
// package tui to avoid an import cycle (tui imports views).
type ViewID int

const (
	ViewAliasList ViewID = iota
	ViewAliasDetails
	ViewAliasCreate
	ViewHelp
	ViewRecipientList
	ViewRecipientDetails
	ViewRecipientAliases
)

// SwitchViewMsg asks the root model to activate another view, pushing the
// current one onto the history stack, optionally delivering a message to the
// next view.
type SwitchViewMsg struct {
	To   ViewID
	With tea.Msg
}

func SwitchView(to ViewID) tea.Cmd {
	return SwitchViewWith(to, nil)
}

// SwitchViewWith is the command the views return to request a switch to
// another view.
func SwitchViewWith(to ViewID, with tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return SwitchViewMsg{To: to, With: with}
	}
}

// PopViewMsg asks the root model to return to the previously
// active view, optionally delivering a message to the restored view.
type PopViewMsg struct {
	With tea.Msg
}

func PopView() tea.Cmd {
	return PopViewWith(nil)
}

// PopViewWith is the command the views return to request a switch to the
// previous view in the history stack.
func PopViewWith(with tea.Msg) tea.Cmd {
	return func() tea.Msg { return PopViewMsg{With: with} }
}

// ViewMsg wraps a command result with the identifier of the view the
// result belongs to, so the root model can deliver it there no matter
// which view is active when the command finishes.
type ViewMsg struct {
	To  ViewID
	Msg tea.Msg
}

// ForView tags every result of cmd for the view with the given id: the
// root model delivers tagged results to that view instead of to
// whichever view is active when the command finishes. Messages the
// root model handles itself (view switches, pops, quits) pass through
// untouched, and batches are unpacked so every command inside keeps
// its tag. Commands must not use tea.Sequence: the runtime's sequence
// messages cannot be re-tagged.
func ForView(id ViewID, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		switch msg := cmd().(type) {
		case nil:
			return nil
		// Root-owned messages pass through unwrapped, so the root
		// model keeps handling navigation and quit.
		case SwitchViewMsg, PopViewMsg, tea.QuitMsg:
			return msg
		// The runtime unpacks BatchMsg before Update sees it; wrap
		// every command inside so none of the results lose their tag.
		case tea.BatchMsg:
			cmds := make([]tea.Cmd, len(msg))
			for i, c := range msg {
				cmds[i] = ForView(id, c)
			}
			return tea.BatchMsg(cmds)
		default:
			return ViewMsg{To: id, Msg: msg}
		}
	}
}
