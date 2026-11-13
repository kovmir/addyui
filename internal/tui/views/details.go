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

// This file implements the TUI screen with the alias details.

// fieldColWidth is the width of the field-name column.
const fieldColWidth = 12

// Row indices of the details table fields;
// they match the table rows built by detailsRows.
const (
	detailsRowEmail = iota
	detailsRowDescription
	detailsRowDomain
	detailsRowLocalPart
	detailsRowActive
	detailsRowPinned
	detailsRowRecipients
	detailsRowForwarded
	detailsRowBlocked
	detailsRowReplied
	detailsRowSent
	detailsRowCreated
	detailsRowUpdated
	detailsRowDeleted
)

// DetailsModel shows the attributes of a single alias as a two-column table.
type DetailsModel struct {
	client  *api.ClientWithResponses
	aliases []api.Alias // Snapshot of the list screen's page.
	index   int         // Position within aliases.
	table   table.Model
	status  string // Result of the last action.
	dirty   bool   // An action succeeded; alias list should refetch.

	// Description text editor.
	descEditor    textinput.Model // Description editor.
	descIsEditing bool

	// Recipients table picker.
	recipientsPicker    table.Model     // Recipients selector.
	recipientsAvail     []recipient     // Available recipients for the picker.
	recipientsSel       map[string]bool // Picked recipients.
	recipientsIsEditing bool
}

// aliasSelectedMsg carries the alias list snapshot and the selected
// index from the list screen to the details screen.
type aliasSelectedMsg struct {
	aliases []api.Alias
	index   int
}

// detailsActionDoneMsg carries the result of a mutating API call made
// from the details screen, plus the refreshed alias on success so the
// snapshot stays in sync with the server.
type detailsActionDoneMsg struct {
	op    string
	alias *api.Alias // fresh alias from the API; nil on error
	err   error
}

func NewDetailsModel(client *api.ClientWithResponses) DetailsModel {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "PROPERTY", Width: fieldColWidth},
			{Title: "VALUE", Width: 40},
		}),
		table.WithFocused(true),
		// The dimensions are updated on first WindowSizeMsg.
		table.WithHeight(15),
		table.WithWidth(80),
		table.WithStyles(tableStyles()),
	)

	input := textinput.New()
	input.Placeholder = "description"
	input.Prompt = "description: "
	input.CharLimit = 200
	// bubbles v2.2.1 bug: textinput does not display .Placeholder unless .width is non-zero.
	// See: https://github.com/charmbracelet/bubbles/issues/779
	input.SetWidth(20)

	p := table.New(
		table.WithColumns([]table.Column{
			{Title: "SEL", Width: 3},
			{Title: "RECIPIENT", Width: 40},
		}),
		table.WithFocused(true),
		table.WithHeight(1), // Grows when recipients arrive.
		table.WithStyles(tableStyles()),
	)

	return DetailsModel{
		client:           client,
		table:            t,
		descEditor:       input,
		recipientsPicker: p,
		recipientsSel:    map[string]bool{},
	}
}

func (m DetailsModel) Init() tea.Cmd {
	return nil
}

func (m DetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		// Let the value column use the rest of the terminal width.
		m.table.SetColumns([]table.Column{
			{Title: "PROPERTY", Width: fieldColWidth},
			{Title: "VALUE", Width: max(msg.Width-fieldColWidth, 20)},
		})
		// Show all rows without scrolling when they fit.
		m.table.SetHeight(min(detailRowCount()+1, max(msg.Height-1, 3)))
		m.descEditor.SetWidth(max(msg.Width-2, 20))
		m.recipientsPicker.SetWidth(msg.Width)
		m.recipientsPicker.SetColumns([]table.Column{
			{Title: "SEL", Width: 4},
			{Title: "RECIPIENT", Width: max(msg.Width-4, 20)},
		})

	case aliasSelectedMsg:
		m.aliases = msg.aliases
		m.index = msg.index
		// A fresh entry: drop any transient state from the last visit.
		m.recipientsIsEditing = false
		m.descIsEditing = false
		m.status = ""
		m.dirty = false
		m.recipientsSel = map[string]bool{}
		// Rows must be set before the cursor: SetCursor clamps against
		// the current row count, so with no rows it would yield -1 and
		// neither the highlight nor the last rows would render.
		m.table.SetRows(detailsRows(&m.aliases[m.index]))
		m.table.SetCursor(0)

	case recipientsLoadedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("load recipients failed: %s", describeError(msg.err))
			return m, nil
		}
		if m.index < 0 || m.index >= len(m.aliases) {
			return m, nil
		}
		m.recipientsAvail = msg.recipients
		// Pre-check the recipients currently attached to the alias.
		// Attached recipients missing from the verified list (possible
		// for unverified ones) cannot be shown in the picker; applying
		// would drop them.
		m.recipientsSel = map[string]bool{}
		for _, r := range aliasRecipients(&m.aliases[m.index]) {
			m.recipientsSel[r.id] = true
		}
		m.recipientsIsEditing = true
		m.syncPicker()

	case detailsActionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %s", msg.op, describeError(msg.err))
			return m, nil
		}
		m.status = msg.op
		m.dirty = true
		if msg.alias != nil {
			m.aliases[m.index] = *msg.alias
		}
		m.syncRows()

	case tea.KeyPressMsg:
		// While the recipients picker is open it consumes all keys,
		// except the two that leave it.
		if m.recipientsIsEditing {
			switch msg.String() {
			case "space":
				if i := m.recipientsPicker.Cursor(); i >= 0 && i < len(m.recipientsAvail) {
					id := m.recipientsAvail[i].id
					m.recipientsSel[id] = !m.recipientsSel[id]
					m.syncPicker()
				}
				return m, nil
			case "enter": // apply the new selection
				m.recipientsIsEditing = false
				return m, m.applyRecipients()
			case "esc": // discard the selection
				m.recipientsIsEditing = false
				return m, nil
			}
			var cmd tea.Cmd
			m.recipientsPicker, cmd = m.recipientsPicker.Update(msg)
			return m, cmd
		}

		// While the description input is open it consumes all keys,
		// except the two that leave it.
		if m.descIsEditing {
			switch msg.String() {
			case "enter": // commit the edited description
				m.descIsEditing = false
				m.descEditor.Blur()
				return m, m.updateDescription(strings.TrimSpace(m.descEditor.Value()))
			case "esc": // cancel editing, keep the old value
				m.descIsEditing = false
				m.descEditor.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.descEditor, cmd = m.descEditor.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		// Display TUI help.
		case "f1":
			return m, SwitchView(ViewHelp)
		case "q":
			return m, tea.Quit
		case "esc":
			if m.dirty {
				// The list shows stale data; have it refetch.
				return m, PopViewWith(listRefreshMsg{index: m.index})
			}
			return m, PopViewWith(listCursorMsg{index: m.index})
		case "p":
			m.move(-1)
			return m, nil
		case "n":
			m.move(+1)
			return m, nil
		case "j":
			m.moveCursorInteractive(+1)
			return m, nil
		case "k":
			m.moveCursorInteractive(-1)
			return m, nil
		case "J":
			m.table.MoveDown(1)
			return m, nil
		case "K":
			m.table.MoveUp(1)
			return m, nil
		case "space":
			return m, m.interact(m.table.Cursor())
		}

		// The arrow keys and every other key go to the table.
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

// move steps to the neighboring alias, clamped to the list bounds.
func (m *DetailsModel) move(delta int) {
	if len(m.aliases) == 0 {
		return
	}
	m.index = min(max(m.index+delta, 0), len(m.aliases)-1)
	m.table.SetCursor(0)
	m.syncRows()
}

// interactiveRowIndices lists the row indices space can act on, in
// display order; they match the rows handled by interact.
var interactiveRowIndices = []int{
	detailsRowDescription,
	detailsRowActive,
	detailsRowPinned,
	detailsRowRecipients,
	detailsRowDeleted,
}

// moveCursorInteractive moves the table cursor to the next interactive
// row, or to the previous one for a negative delta. It clamps at the
// first and last interactive rows, so the cursor never rests on a row
// space cannot act on.
func (m *DetailsModel) moveCursorInteractive(delta int) {
	cur := m.table.Cursor()
	next := cur
	if delta > 0 {
		for _, row := range interactiveRowIndices {
			if row > cur {
				next = row
				break
			}
		}
	} else {
		for i := len(interactiveRowIndices) - 1; i >= 0; i-- {
			if row := interactiveRowIndices[i]; row < cur {
				next = row
				break
			}
		}
	}
	m.table.SetCursor(next)
}

// interact starts the interaction of the row under the cursor: editing
// for the description, a picker for the recipients, and an API toggle
// for the active, pinned and deleted states. Non-interactive rows
// (email, domain, local part, stats, timestamps) do nothing.
func (m *DetailsModel) interact(row int) tea.Cmd {
	if m.index < 0 || m.index >= len(m.aliases) {
		return nil
	}
	a := &m.aliases[m.index]
	switch row {
	case detailsRowDescription:
		m.descIsEditing = true
		m.descEditor.SetValue(str(a.Description))
		cmd := m.descEditor.Focus()
		m.descEditor.CursorEnd()
		return cmd
	case detailsRowActive:
		return m.toggleActive(a)
	case detailsRowPinned:
		return m.togglePin(a)
	case detailsRowRecipients:
		// The picker needs the verified recipients; the alias's own
		// attached recipients come from the snapshot.
		return fetchVerifiedRecipients(m.client)
	case detailsRowDeleted:
		return m.toggleDeleted(a)
	}
	return nil
}

// updateDescription sends the new description to the API, then refreshes
// the snapshot. Sending nothing when the value is unchanged avoids a
// pointless request.
func (m *DetailsModel) updateDescription(description string) tea.Cmd {
	if m.index < 0 || m.index >= len(m.aliases) {
		return nil
	}
	a := &m.aliases[m.index]
	id := str(a.Id)
	if id == "" || description == str(a.Description) {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.UpdateASpecificAliasWithResponse(ctx, id,
			&api.UpdateASpecificAliasParams{},
			api.UpdateASpecificAliasJSONRequestBody{Description: &description})
		if err = apiError(err, resp); err != nil {
			return detailsActionDoneMsg{op: "update description", err: err}
		}
		return m.refreshMsg(ctx, id, "updated description")
	}
}

// toggleActive activates the alias if it is inactive, and vice versa.
// A deleted alias cannot be activated; restoring it re-activates it
// server-side, so space sends a restore only.
func (m *DetailsModel) toggleActive(a *api.Alias) tea.Cmd {
	id := str(a.Id)
	if id == "" {
		return nil
	}
	if a.DeletedAt != nil {
		return m.restoreCmd(id, "restored alias")
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
			return detailsActionDoneMsg{op: "toggle active", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled active state")
	}
}

// togglePin pins the alias if it is unpinned, and vice versa.
func (m *DetailsModel) togglePin(a *api.Alias) tea.Cmd {
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
			return detailsActionDoneMsg{op: "toggle pin", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled pin state")
	}
}

// toggleDeleted restores the alias if it is deleted, and deletes it
// otherwise. Deletion is soft: the alias can be restored later.
func (m *DetailsModel) toggleDeleted(a *api.Alias) tea.Cmd {
	id := str(a.Id)
	if id == "" {
		return nil
	}
	if a.DeletedAt != nil {
		return m.restoreCmd(id, "restored alias")
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.DeleteASpecificAliasWithResponse(ctx, id,
			&api.DeleteASpecificAliasParams{})
		if err = apiError(err, resp); err != nil {
			return detailsActionDoneMsg{op: "delete alias", err: err}
		}
		return m.refreshMsg(ctx, id, "deleted alias")
	}
}

// restoreCmd restores a deleted alias and refreshes the snapshot.
func (m *DetailsModel) restoreCmd(id, op string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.RestoreASpecificDeletedAliasWithResponse(ctx, id,
			&api.RestoreASpecificDeletedAliasParams{})
		if err = apiError(err, resp); err != nil {
			return detailsActionDoneMsg{op: op, err: err}
		}
		return m.refreshMsg(ctx, id, op)
	}
}

// applyRecipients sends the picker's selection to the API, replacing
// the alias's attached recipients, then refreshes the snapshot.
func (m *DetailsModel) applyRecipients() tea.Cmd {
	if m.index < 0 || m.index >= len(m.aliases) {
		return nil
	}
	id := str(m.aliases[m.index].Id)
	if id == "" {
		return nil
	}
	ids := make([]string, 0, len(m.recipientsSel))
	for _, r := range m.recipientsAvail {
		if m.recipientsSel[r.id] {
			ids = append(ids, r.id)
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.UpdateRecipientsForASpecificAliasWithResponse(ctx,
			&api.UpdateRecipientsForASpecificAliasParams{},
			api.UpdateRecipientsForASpecificAliasJSONRequestBody{
				AliasId:      id,
				RecipientIds: ids,
			})
		if err = apiError(err, resp); err != nil {
			return detailsActionDoneMsg{op: "update recipients", err: err}
		}
		return m.refreshMsg(ctx, id, "updated recipients")
	}
}

// refreshMsg fetches the alias and returns a detailsActionDoneMsg
// carrying the fresh snapshot, so the table reflects the server state.
func (m *DetailsModel) refreshMsg(ctx context.Context, id, op string) tea.Msg {
	resp, err := m.client.GetASpecificAliasWithResponse(ctx, id, &api.GetASpecificAliasParams{})
	if err != nil {
		return detailsActionDoneMsg{op: op, err: fmt.Errorf("refresh failed: %w", err)}
	}
	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return detailsActionDoneMsg{op: op, err: fmt.Errorf("refresh failed: %s", resp.Status())}
	}
	// The generated response embeds an anonymous struct with the same
	// layout as api.Alias; convert it into the snapshot type.
	alias := api.Alias(*resp.JSON200.Data)
	return detailsActionDoneMsg{op: op, alias: &alias}
}

// syncPicker refreshes the recipients picker rows and height after a
// recipient was toggled or the list was fetched.
func (m *DetailsModel) syncPicker() {
	rows := make([]table.Row, 0, len(m.recipientsAvail))
	for _, r := range m.recipientsAvail {
		selected := ""
		if m.recipientsSel[r.id] {
			selected = "+"
		}
		rows = append(rows, table.Row{selected, r.email})
	}
	m.recipientsPicker.SetRows(rows)
	m.recipientsPicker.SetHeight(len(rows) + 1) // header + rows
}

// syncRows refreshes the table rows after the alias snapshot changed.
func (m *DetailsModel) syncRows() {
	if m.index < 0 || m.index >= len(m.aliases) {
		return
	}
	m.table.SetRows(detailsRows(&m.aliases[m.index]))
}

func (m DetailsModel) View() tea.View {
	var b strings.Builder
	b.WriteString(m.table.View())
	if m.status != "" {
		fmt.Fprintf(&b, " - %s", m.status)
	}

	if m.descIsEditing {
		fmt.Fprintf(&b, "\n\n%s", m.descEditor.View())
	} else if m.recipientsIsEditing {
		fmt.Fprintf(&b, "\n\n%s", m.recipientsPicker.View())
	} else {
		if len(m.aliases) > 0 {
			fmt.Fprintf(&b, "\n\n%d/%d aliases", m.index+1, len(m.aliases))
		}
	}
	return tea.NewView(b.String())
}

// detailRowCount returns the number of rows the details table renders.
func detailRowCount() int {
	return len(detailsRows(&api.Alias{}))
}

// detailsRows converts an alias into rows for the details table. The
// row order matches the drowXxx constants.
func detailsRows(a *api.Alias) []table.Row {
	deletedAt := str(a.DeletedAt)
	if deletedAt == "" {
		deletedAt = "-"
	}
	description := str(a.Description)
	if description == "" {
		description = "-"
	}
	attached := aliasRecipients(a)
	emails := make([]string, 0, len(attached))
	for _, r := range attached {
		emails = append(emails, r.email)
	}

	return []table.Row{
		{"Email", str(a.Email)},
		{"Description", description},
		{"Domain", str(a.Domain)},
		{"Local part", str(a.LocalPart)},
		{"Active", fmt.Sprintf("%v", a.Active != nil && *a.Active)},
		{"Pinned", fmt.Sprintf("%v", a.Pinned != nil && *a.Pinned)},
		{"Recipients", recipientSummary(emails)},
		{"Forwarded", count(a.EmailsForwarded)},
		{"Blocked", count(a.EmailsBlocked)},
		{"Replied", count(a.EmailsReplied)},
		{"Sent", count(a.EmailsSent)},
		{"Created", str(a.CreatedAt)},
		{"Updated", str(a.UpdatedAt)},
		{"Deleted", deletedAt},
	}
}
