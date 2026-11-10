// Package tui holds the root Bubble Tea model that owns the views and
// routes messages between them.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kovmir/addyui/internal/api"
	"github.com/kovmir/addyui/internal/tui/views"
)

// AppModel is the root model. It owns every view, routes messages to the
// active one, and handles navigation when the views request it.
type AppModel struct {
	// Currently active view.
	active views.ViewID
	// When the app switches to another view, it pushes the current one
	// onto the history stack so it is possible to return to it.
	history []views.ViewID
	views   map[views.ViewID]tea.Model
}

// appOptions collects the optional startup state set through AppOption.
type appOptions struct {
	searchTerm string            // Pre-applied server-side search term for the alias list.
	listFilter views.StateFilter // Default alias list view filter.
}

// AppOption configures optional application startup state.
type AppOption func(*appOptions)

func WithSearch(searchTerm string) AppOption {
	return func(o *appOptions) { o.searchTerm = searchTerm }
}

func WithState(listFilter views.StateFilter) AppOption {
	return func(o *appOptions) { o.listFilter = listFilter }
}

func NewAppModel(client *api.ClientWithResponses) AppModel {
	return NewAppModelWithView(client, views.ViewList)
}

func NewAppModelWithView(client *api.ClientWithResponses, activeView views.ViewID, opts ...AppOption) AppModel {
	var o appOptions
	for _, opt := range opts {
		opt(&o)
	}
	return AppModel{
		active: activeView,
		views: map[views.ViewID]tea.Model{
			views.ViewList:    views.NewListModel(client, o.searchTerm, o.listFilter),
			views.ViewDetails: views.NewDetailsModel(client),
			views.ViewCreate:  views.NewCreateModel(client),
			views.ViewHelp:    views.NewHelpModel(),
		},
	}
}

// Init renders only the currently active view.
func (m AppModel) Init() tea.Cmd {
	return m.views[m.active].Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Exit the app.
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	// Switch currently active view.
	case views.SwitchViewMsg:
		if msg.To != m.active {
			// Push the current view onto the history stack.
			m.history = append(m.history, m.active)
			// Switch to the next view.
			m.active = msg.To
			// Deliver the payload first...
			var cmd tea.Cmd
			m, cmd = m.updateChild(m.active, msg.With)
			// ... then run the view's Init().
			return m, tea.Batch(cmd, m.views[m.active].Init())
		}
		return m, nil
	// Return to the previous view.
	case views.PopViewMsg:
		// Quit if there is no previous view.
		if len(m.history) == 0 {
			return m, tea.Quit
		}
		// Pop the view from history.
		m.active = m.history[len(m.history)-1]
		m.history = m.history[:len(m.history)-1]
		// Deliver the payload.
		return m.updateChild(m.active, msg.With)
	// Send window resize message to all views.
	case tea.WindowSizeMsg:
		var cmds []tea.Cmd
		var cmd tea.Cmd
		for k := range m.views {
			m, cmd = m.updateChild(k, msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Delegate everything else to the active view only.
	return m.updateChild(m.active, msg)
}

// updateChild delivers the message to the child view.
func (m AppModel) updateChild(id views.ViewID, msg tea.Msg) (AppModel, tea.Cmd) {
	if msg == nil {
		return m, nil
	}
	updated, cmd := m.views[id].Update(msg)
	m.views[id] = updated
	return m, cmd
}

// View renders only the currently active view.
func (m AppModel) View() tea.View {
	return m.views[m.active].View()
}
