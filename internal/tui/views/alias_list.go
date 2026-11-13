package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kovmir/addyui/internal/api"
)

// This file implements the TUI screen with the alias list.

const pageSize = 100 // API max.

// defaultTableHeight is used until the first WindowSizeMsg arrives.
const defaultTableHeight = 24

// pinFilter selects which aliases are listed based on pin state.
type pinFilter int

const (
	pinAll pinFilter = iota
	pinPinned
	pinUnpinned
)

// label returns the display name of the pin filter.
func (f pinFilter) label() string {
	switch f {
	case pinAll:
		return "all"
	case pinPinned:
		return "pinned"
	case pinUnpinned:
		return "unpinned"
	default:
		return fmt.Sprintf("unknown pin filter %d", f)
	}
}

// StateFilter selects which aliases are listed based on their life cycle.
type StateFilter int

const (
	StateActiveInactive StateFilter = iota // Default: active and inactive.
	StateAll                               // Everything, including deleted.
	StateActive                            // Active only.
	StateDeleted                           // Deleted only.
)

// cycleOrder defines the order in which the state filter cycles.
var cycleOrder = []StateFilter{StateAll, StateActiveInactive, StateActive, StateDeleted}

// label returns the display name of the state filter.
func (f StateFilter) label() string {
	switch f {
	case StateActiveInactive:
		return "active + inactive"
	case StateAll:
		return "all"
	case StateActive:
		return "active"
	case StateDeleted:
		return "deleted"
	default:
		return fmt.Sprintf("unknown state filter %d", f)
	}
}

// aliasesLoadedMsg carries the result of an API list call back to the model.
type aliasesLoadedMsg struct {
	aliases  []api.Alias
	page     int
	lastPage int
	total    int
	err      error
}

// actionDoneMsg carries the result of a mutating API call (activate,
// deactivate, pin, unpin) back to the model.
type actionDoneMsg struct {
	op  string
	err error
}

// listCursorMsg moves the table cursor to the given row index.
type listCursorMsg struct{ index int }

// listRefreshMsg moves the table cursor to the given row index and
// refetches. The details screen sends it when popping back after it
// changed the alias, so the list reflects those changes.
type listRefreshMsg struct{ index int }

// listForRecipientMsg restricts the alias list to the aliases attached
// to one recipient. The recipient details screen sends it when the user
// opens the alias list from the "Aliases" row.
type listForRecipientMsg struct {
	recipientID string
	email       string
}

// ListModel is the alias list screen.
type ListModel struct {
	client  *api.ClientWithResponses
	aliases []api.Alias
	table   table.Model
	loading bool // Currently fetching alias list?
	err     error
	status  string // Result of the last action.

	// Both are 0 until the first WindowSizeMsg.
	termHeight int
	termWidth  int

	page     int // 1-based page number
	lastPage int // From the API's pagination meta; 0 until first load.
	total    int // Total number of aliases matching the current filters.

	pinFilter   pinFilter
	stateFilter StateFilter

	// Non-empty: only show the aliases attached to this recipient.
	recipientFilter string
	recipientEmail  string // Recipient email shown in the status line.

	searchTerm  string // Server-side search term.
	searching   bool   // Whether the search field is active.
	searchInput textinput.Model

	// The user must hit the forget key twice in a row to forget an alias.
	forgetArm  bool   // Has the use pressed the forget key once?
	forgetName string // Alias email shown in the forget confirmation prompt.
}

// updateTableSize resizes the table to fit the terminal, accounting for
// the extra lines for the search field and status.
func (m *ListModel) updateTableSize() {
	chromeLines := 3 // Blank line, search or alias count, status.
	m.table.SetHeight(max(m.termHeight-chromeLines, 3))
	m.table.SetWidth(max(m.termWidth, 20))
}

// NewListModel builds the alias list screen.
func NewListModel(client *api.ClientWithResponses, initialSearch string, initialFilter StateFilter) ListModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "email or description"
	searchInput.Prompt = "search: "
	searchInput.CharLimit = 200
	// bubbles v2.2.1 bug: textinput does not display .Placeholder unless .width is non-zero.
	// See: https://github.com/charmbracelet/bubbles/issues/779
	searchInput.SetWidth(20)

	km := table.DefaultKeyMap()
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "PIN", Width: 3},
			{Title: "ACT", Width: 3},
			{Title: "EMAIL", Width: 38},
			{Title: "DESCRIPTION", Width: 30},
			{Title: "FWD", Width: 4},
			{Title: "BLK", Width: 4},
		}),
		table.WithKeyMap(km),
		table.WithStyles(tableStyles()),
		table.WithFocused(true),
		table.WithHeight(defaultTableHeight),
	)

	return ListModel{
		client:      client,
		loading:     true,
		table:       t,
		page:        1,
		stateFilter: initialFilter,
		pinFilter:   pinAll,
		searchTerm:  initialSearch,
		searchInput: searchInput,
	}
}

func (m ListModel) Init() tea.Cmd {
	// Kick off the first fetch.
	return m.fetchAliases()
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Any key other than "x" cancels a pending forget confirmation,
		// so forgetting requires two "x" presses in a row.
		if m.searching || msg.String() != "x" {
			m.forgetArm = false
		}

		// While the search input is open it consumes all the keys.
		if m.searching {
			switch msg.String() {
			case "enter":
				m.searchTerm = strings.TrimSpace(m.searchInput.Value())
				m.searching = false
				m.searchInput.Blur()
				m.updateTableSize()
				m.table.SetCursor(0)
				m.page = 1 // A new search starts from the first page.
				return m, m.fetchAliases()
			case "esc":
				m.searching = false
				m.searchInput.SetValue(m.searchTerm) // Restore the applied term.
				m.searchInput.Blur()
				m.updateTableSize()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		// Quit TUI.
		case "q":
			return m, tea.Quit
		// Go to the previous view.
		case "esc":
			return m, PopView()
		// Open alias details.
		case "enter":
			if _, ok := m.selected(); ok {
				return m, SwitchViewWith(ViewDetails,
					aliasSelectedMsg{aliases: m.aliases, index: m.table.Cursor()})
			}
		// Display TUI help.
		case "f1":
			return m, SwitchView(ViewHelp)
		// Create a new alias.
		case "c":
			return m, SwitchView(ViewCreate)
		// Manage recipients.
		case "R":
			return m, SwitchView(ViewRecipientList)
		// Next API alias page.
		case "right", "l":
			if m.page < m.lastPage {
				m.page++
				m.table.SetCursor(0)
				return m, m.fetchAliases()
			}
		// Previous API alias page.
		case "left", "h":
			if m.page > 1 {
				m.page--
				m.table.SetCursor(0)
				return m, m.fetchAliases()
			}
		// Toggle alias itself.
		case "space":
			if a, ok := m.selected(); ok {
				return m, m.toggleActive(a)
			}
		// Toggle alias pin.
		case "p":
			if a, ok := m.selected(); ok {
				return m, m.togglePin(a)
			}
		// Toggle alias delete.
		case "d":
			if a, ok := m.selected(); ok {
				return m, m.toggleDeleted(a)
			}
		// Forget an alias.
		case "x":
			a, ok := m.selected()
			if !ok {
				m.forgetArm = false
				return m, nil
			}
			if !m.forgetArm {
				m.forgetArm = true
				m.forgetName = str(a.Email)
				return m, nil
			}
			m.forgetArm = false
			return m, m.forgetAlias(a)
		// Cycle pin filter.
		case "f":
			m.pinFilter = (m.pinFilter + 1) % 3
			return m, m.fetchAliases()
		// Cycle view filter.
		case "v":
			for i, f := range cycleOrder {
				if f == m.stateFilter {
					m.stateFilter = cycleOrder[(i+1)%len(cycleOrder)]
					break
				}
			}
			return m, m.fetchAliases()
		// Refetch aliases.
		case "r":
			return m, m.fetchAliases()
		// Alias API search.
		case "/":
			m.searching = true
			m.searchInput.SetValue(m.searchTerm)
			cmd := m.searchInput.Focus()
			m.searchInput.CursorEnd()
			m.updateTableSize()
			return m, cmd
		}
		// The rest is consumed by the table...
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.termHeight = msg.Height
		m.termWidth = msg.Width
		m.updateTableSize()

	case aliasesLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.forgetArm = false
		m.aliases = msg.aliases
		m.table.SetRows(aliasRows(msg.aliases))
		// Keep the cursor position across refetches, but clamp it back
		// into range when the page shrank or had no rows yet.
		if c := m.table.Cursor(); c < 0 || c >= len(m.aliases) {
			m.table.SetCursor(0)
		}
		m.page = msg.page
		m.lastPage = msg.lastPage
		m.total = msg.total

	case listCursorMsg:
		m.table.SetCursor(msg.index)

	case listRefreshMsg:
		m.table.SetCursor(msg.index)
		return m, m.fetchAliases()

	case listForRecipientMsg:
		// Init() runs right after this message and kicks off the
		// filtered fetch, so no command is returned here. The cursor
		// is left alone: with no rows yet, SetCursor would clamp to -1.
		m.recipientFilter = msg.recipientID
		m.recipientEmail = msg.email
		m.page = 1

	case actionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %s", msg.op, describeError(msg.err))
			return m, nil
		}
		m.status = msg.op
		return m, m.fetchAliases() // Refetch aliases after actions.
	}
	return m, nil
}

// selected returns the alias under the table cursor, if any.
func (m ListModel) selected() (api.Alias, bool) {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.aliases) {
		return api.Alias{}, false
	}
	return m.aliases[i], true
}

func (m ListModel) View() tea.View {
	var b strings.Builder

	switch {
	case m.err != nil:
		fmt.Fprintf(&b, "Error: %s\n\n", describeError(m.err))
	case m.loading:
		b.WriteString("Loading aliases...\n")
	case len(m.aliases) == 0:
		b.WriteString("No aliases found.\n")
	default:
		b.WriteString(m.table.View())
	}

	if m.searching {
		fmt.Fprintf(&b, "\n\n%s", m.searchInput.View())
	} else {
		fmt.Fprintf(&b, "\n\n%d/%d aliases", m.table.Cursor()+1, len(m.aliases))
	}

	if m.forgetArm {
		// The pending forget confirmation replaces the whole status line
		// so it can't be missed.
		fmt.Fprintf(&b, "\npress x again to FORGET %s permanently (cannot be undone)", m.forgetName)
	} else {
		fmt.Fprintf(&b, "\npage %d/%d", m.page, m.lastPage)
		if m.total > 0 {
			fmt.Fprintf(&b, " (%d total)", m.total)
		}
		fmt.Fprintf(&b, " - view: %s - pins: %s", m.stateFilter.label(), m.pinFilter.label())
		if m.recipientFilter != "" {
			fmt.Fprintf(&b, " - recipient: %s", m.recipientEmail)
		}
		if m.searchTerm != "" {
			fmt.Fprintf(&b, " - search: %q", m.searchTerm)
		}
		if m.status != "" {
			fmt.Fprintf(&b, " - %s", m.status)
		}
	}

	return tea.NewView(b.String())
}

// aliasRows converts API aliases into table rows.
func aliasRows(aliases []api.Alias) []table.Row {
	rows := make([]table.Row, 0, len(aliases))
	for _, a := range aliases {
		pin, act := " ", " "
		if a.Pinned != nil && *a.Pinned {
			pin = "^"
		}
		if a.Active != nil && *a.Active {
			act = "+"
		}
		if a.DeletedAt != nil {
			act = "X"
		}
		rows = append(rows, table.Row{
			pin,
			act,
			str(a.Email),
			str(a.Description),
			count(a.EmailsForwarded),
			count(a.EmailsBlocked),
		})
	}
	return rows
}

// fetchAliases fetches a lists of aliases using the model's
// current pin and state filters.
func (m ListModel) fetchAliases() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		params := &api.GetAllAliasesParams{
			PageSize:   new(int),
			PageNumber: new(int),
		}
		*params.PageSize = pageSize
		*params.PageNumber = m.page

		switch m.pinFilter {
		case pinPinned:
			t := "true"
			params.FilterPinned = &t
		case pinUnpinned:
			f := "false"
			params.FilterPinned = &f
		}

		switch m.stateFilter {
		case StateAll:
			w := "with"
			params.FilterDeleted = &w
		case StateActive:
			t := "true"
			params.FilterActive = &t
		case StateDeleted:
			o := "only"
			params.FilterDeleted = &o
		}
		// stateActiveInactive is the API default, thus no handler.

		if m.searchTerm != "" {
			params.FilterSearch = &m.searchTerm
		}

		// Ask for the recipients list so the details view can show
		// them without another API call.
		with := "recipients"
		params.With = &with

		// Restrict the list to one recipient's aliases when the
		// recipient details screen asked for it.
		if m.recipientFilter != "" {
			params.Recipient = &m.recipientFilter
		}

		resp, err := m.client.GetAllAliasesWithResponse(ctx, params)
		if err != nil {
			return aliasesLoadedMsg{err: fmt.Errorf("request failed: %w", err)}
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			return aliasesLoadedMsg{err: fmt.Errorf("unexpected response: %s", resp.Status())}
		}
		msg := aliasesLoadedMsg{aliases: *resp.JSON200.Data, page: m.page}
		if meta := resp.JSON200.Meta; meta != nil {
			if meta.LastPage != nil {
				msg.lastPage = *meta.LastPage
			}
			if meta.Total != nil {
				msg.total = *meta.Total
			}
		}
		return msg
	}
}

// toggleActive activates the alias if it is inactive, and vice versa.
func (m ListModel) toggleActive(a api.Alias) tea.Cmd {
	id := str(a.Id)
	if id == "" {
		return nil
	}
	activate := a.Active == nil || !*a.Active
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if activate {
			var resp *api.ActivateASpecificAliasResponse
			resp, err = m.client.ActivateASpecificAliasWithResponse(ctx,
				&api.ActivateASpecificAliasParams{},
				api.ActivateASpecificAliasJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DeactivateASpecificAliasResponse
			resp, err = m.client.DeactivateASpecificAliasWithResponse(ctx, id,
				&api.DeactivateASpecificAliasParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return actionDoneMsg{op: "toggle active", err: err}
		}
		return actionDoneMsg{op: "toggled active state"}
	}
}

// togglePin pins the alias if it is unpinned, and vice versa.
func (m ListModel) togglePin(a api.Alias) tea.Cmd {
	id := str(a.Id)
	if id == "" {
		return nil
	}
	pin := a.Pinned == nil || !*a.Pinned
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if pin {
			var resp *api.PinASpecificAliasResponse
			resp, err = m.client.PinASpecificAliasWithResponse(ctx,
				&api.PinASpecificAliasParams{},
				api.PinASpecificAliasJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.UnpinASpecificAliasResponse
			resp, err = m.client.UnpinASpecificAliasWithResponse(ctx, id,
				&api.UnpinASpecificAliasParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return actionDoneMsg{op: "toggle pin", err: err}
		}
		return actionDoneMsg{op: "toggled pin state"}
	}
}

// toggleDeleted restores the alias if it is deleted, and vice versa.
func (m ListModel) toggleDeleted(a api.Alias) tea.Cmd {
	id := str(a.Id)
	if id == "" {
		return nil
	}
	restore := a.DeletedAt != nil
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if restore {
			var resp *api.RestoreASpecificDeletedAliasResponse
			resp, err = m.client.RestoreASpecificDeletedAliasWithResponse(ctx, id,
				&api.RestoreASpecificDeletedAliasParams{})
			err = apiError(err, resp)
		} else {
			var resp *api.DeleteASpecificAliasResponse
			resp, err = m.client.DeleteASpecificAliasWithResponse(ctx, id,
				&api.DeleteASpecificAliasParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return actionDoneMsg{op: "toggle deleted", err: err}
		}
		if restore {
			return actionDoneMsg{op: "restored alias"}
		}
		return actionDoneMsg{op: "deleted alias"}
	}
}

// forgetAlias permanently forgets the alias.
func (m ListModel) forgetAlias(a api.Alias) tea.Cmd {
	id := str(a.Id)
	if id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.ForgetASpecificAliasWithResponse(ctx, id,
			&api.ForgetASpecificAliasParams{})
		if err := apiError(err, resp); err != nil {
			return actionDoneMsg{op: "forget alias", err: err}
		}
		return actionDoneMsg{op: "forgot alias"}
	}
}
