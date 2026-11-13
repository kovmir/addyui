package views

import tea "charm.land/bubbletea/v2"

// ViewID identifies a screen/view in the app. Defined here rather than in
// package tui to avoid an import cycle (tui imports views).
type ViewID int

const (
	ViewList ViewID = iota
	ViewDetails
	ViewCreate
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
