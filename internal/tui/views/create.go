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

// This file implements the TUI screen to create a new alias.

// aliasFormat is one alias format option accepted by the API's create
// endpoint (CreateNewAliasJSONBody.Format).
type aliasFormat struct {
	value string // value sent to the API
	label string // display name in the table
}

// aliasFormats is the cycle order for the format row; it matches the
// options listed in the API spec, with the account's likely default
// (random_words) in a handy position.
var aliasFormats = []aliasFormat{
	{"random_words", "random words"},
	{"random_characters", "random characters"},
	{"uuid", "uuid"},
	{"random_male_name", "random male name"},
	{"random_female_name", "random female name"},
	{"random_noun", "random noun"},
	{"custom", "custom"},
}

// Row indices of the selectable fields; they match the table rows.
const (
	createRowDomain = iota
	createRowFormat
	createRowRecipients
	createRowDescription
)

// createRowCount is the number of rows in the field table.
const createRowCount = 4

// createOptionsMsg carries the result of the domain-options fetch.
type createOptionsMsg struct {
	domains       []string
	defaultDomain string
	defaultFormat string
	err           error
}

// createDoneMsg carries the result of the create-alias call back to
// the model.
type createDoneMsg struct {
	err error
}

// CreateModel is the "new alias" screen. It presents the editable
// fields as a table; the cursor selects a row and space starts the
// interaction: cycling through the defined options for domain and
// format, opening a single-line input for the description, or showing
// a recipients table below in which space toggles each recipient.
type CreateModel struct {
	client *api.ClientWithResponses
	table  table.Model
	picker table.Model // recipients selection; visible while pickerOpen
	input  textinput.Model

	domains     []string
	domainIdx   int
	formatIdx   int
	recipients  []recipient
	selected    map[string]bool // recipients chosen for the new alias, by id
	description string

	pickerOpen bool
	editing    int // row being edited; -1 when none

	loading bool
	err     error
	status  string
}

// NewCreateModel builds the create screen. It stays idle until a
// createStartMsg arrives.
func NewCreateModel(client *api.ClientWithResponses) CreateModel {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "PROPERTY", Width: fieldColWidth},
			{Title: "VALUE", Width: 40},
		}),
		table.WithHeight(createRowCount+1), // header + rows, replaced on WindowSizeMsg
		table.WithWidth(80),                // replaced on WindowSizeMsg
		table.WithFocused(true),
		table.WithStyles(tableStyles()),
	)

	input := textinput.New()
	input.Placeholder = "value"
	input.Prompt = "edit: "
	input.CharLimit = 200
	// bubbles v2.2.1 bug: textinput does not display .Placeholder unless .width is non-zero.
	// See: https://github.com/charmbracelet/bubbles/issues/779
	input.SetWidth(20)

	p := table.New(
		table.WithColumns([]table.Column{
			{Title: "SEL", Width: 4},
			{Title: "RECIPIENT", Width: 40},
		}),
		table.WithFocused(true),
		table.WithHeight(1), // grows with the rows on syncPicker
		table.WithStyles(tableStyles()),
	)

	m := CreateModel{
		client:   client,
		table:    t,
		picker:   p,
		input:    input,
		selected: map[string]bool{},
		editing:  -1,
		loading:  true,
		err:      nil,
		status:   "",
	}
	m.table.SetRows(m.rows())
	return m
}

func (m CreateModel) Init() tea.Cmd {
	return tea.Batch(m.fetchDomains(), fetchVerifiedRecipients(m.client))
}

func (m CreateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		// Let the value column use the rest of the terminal width.
		m.table.SetColumns([]table.Column{
			{Title: "PROPERTY", Width: fieldColWidth},
			{Title: "VALUE", Width: max(msg.Width-fieldColWidth, 20)},
		})
		m.table.SetHeight(createRowCount + 1)
		m.input.SetWidth(max(msg.Width-2, 20))
		m.picker.SetWidth(msg.Width)
		m.picker.SetColumns([]table.Column{
			{Title: "SEL", Width: 4},
			{Title: "RECIPIENT", Width: max(msg.Width-4, 20)},
		})

	case createOptionsMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		if len(msg.domains) > 0 {
			m.domains = msg.domains
			m.domainIdx = 0
			for i, d := range msg.domains {
				if d == msg.defaultDomain {
					m.domainIdx = i
					break
				}
			}
		}
		m.formatIdx = 0
		for i, f := range aliasFormats {
			if f.value == msg.defaultFormat {
				m.formatIdx = i
				break
			}
		}
		m.syncRows()

	case verifiedRecipientsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.recipients = msg.recipients
		m.syncPicker()
		m.syncRows()

	case createDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("create failed: %s", describeError(msg.err))
			return m, nil
		}
		return m, PopViewWith(actionDoneMsg{op: "alias created"})

	case tea.KeyPressMsg:
		// While the recipients picker is open it consumes all keys,
		// except the two that leave it.
		if m.pickerOpen {
			switch msg.String() {
			case "space":
				if i := m.picker.Cursor(); i >= 0 && i < len(m.recipients) {
					id := m.recipients[i].id
					m.selected[id] = !m.selected[id]
					m.syncPicker()
					m.syncRows()
				}
				return m, nil
			case "esc", "enter":
				m.pickerOpen = false
				return m, nil
			}
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			return m, cmd
		}

		// While the input is open it consumes all keys, except the
		// two that leave it.
		if m.editing >= 0 {
			switch msg.String() {
			case "enter": // commit the edited value
				m.description = strings.TrimSpace(m.input.Value())
				m.editing = -1
				m.input.Blur()
				m.syncRows()
				return m, nil
			case "esc": // cancel editing, keep the old value
				m.editing = -1
				m.input.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		// Display TUI help.
		case "f1":
			return m, SwitchView(ViewHelp)
		case "q":
			return m, tea.Quit
		case "esc":
			return m, PopView()
		case "space":
			return m, m.interact(m.table.Cursor())
		case "enter":
			if cmd := m.submit(); cmd != nil {
				return m, cmd
			}
		}

		// j/k and friends move the table cursor.
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

// interact starts the interaction of the row under the cursor: cycling
// for the option rows, editing for the text rows, and opening the
// recipients picker for the recipients row.
func (m *CreateModel) interact(row int) tea.Cmd {
	switch row {
	case createRowDomain:
		m.cycle(createRowDomain, +1)
	case createRowFormat:
		m.cycle(createRowFormat, +1)
	case createRowRecipients:
		m.pickerOpen = true
	case createRowDescription:
		m.editing = row
		m.input.SetValue(m.description)
		m.input.Prompt = "description: "
		cmd := m.input.Focus()
		m.input.CursorEnd()
		return cmd
	}
	return nil
}

// View renders the table, the status line and, while the input is
// open, the input itself.
func (m CreateModel) View() tea.View {
	var b strings.Builder
	b.WriteString(m.table.View())

	if m.status != "" {
		fmt.Fprintf(&b, "\n\n%s", m.status)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\n\nError: %s", describeError(m.err))
	}
	if m.loading {
		b.WriteString("\n\nLoading domain options...")
	}

	if m.editing >= 0 {
		fmt.Fprintf(&b, "\n\n%s", m.input.View())
	} else if m.pickerOpen {
		fmt.Fprintf(&b, "\n\n%s", m.picker.View())
	}
	return tea.NewView(b.String())
}

// rows builds the field table rows from the current field values.
func (m CreateModel) rows() []table.Row {
	recipients := m.recipientsSummary()
	description := m.description
	if description == "" {
		description = "-"
	}
	return []table.Row{
		{"Domain", m.currentDomain()},
		{"Format", aliasFormats[m.formatIdx].label},
		{"Recipients", recipients},
		{"Description", description},
	}
}

// recipientsSummary renders the recipient selection for the field
// table; "-" means none are selected and the API uses the default
// recipient.
func (m CreateModel) recipientsSummary() string {
	var emails []string
	for _, r := range m.recipients {
		if m.selected[r.id] {
			emails = append(emails, r.email)
		}
	}
	return recipientSummary(emails)
}

// syncPicker refreshes the recipients picker rows and height after a
// recipient was toggled or the list was fetched.
func (m *CreateModel) syncPicker() {
	rows := make([]table.Row, 0, len(m.recipients))
	for _, r := range m.recipients {
		selected := ""
		if m.selected[r.id] {
			selected = "+"
		}
		rows = append(rows, table.Row{selected, r.email})
	}
	m.picker.SetRows(rows)
	m.picker.SetHeight(len(rows) + 1) // header + rows
}

// syncRows refreshes the table rows after a value changed.
func (m *CreateModel) syncRows() {
	m.table.SetRows(m.rows())
}

// currentDomain returns the selected domain, or "" when the domain
// options are not (yet) loaded.
func (m CreateModel) currentDomain() string {
	if m.domainIdx < 0 || m.domainIdx >= len(m.domains) {
		return ""
	}
	return m.domains[m.domainIdx]
}

// cycle moves the domain or format field to the next option, wrapping
// around at the end of the list.
func (m *CreateModel) cycle(row, delta int) {
	switch row {
	case createRowDomain:
		if len(m.domains) == 0 {
			return
		}
		m.domainIdx = ((m.domainIdx+delta)%len(m.domains) + len(m.domains)) % len(m.domains)
	case createRowFormat:
		m.formatIdx = ((m.formatIdx+delta)%len(aliasFormats) + len(aliasFormats)) % len(aliasFormats)
	}
	m.syncRows()
}

// submit validates the form and returns the command that creates the
// alias, or nil when the form is not ready.
func (m CreateModel) submit() tea.Cmd {
	if m.loading || m.currentDomain() == "" {
		return nil
	}
	format := aliasFormats[m.formatIdx].value
	domain := m.currentDomain()
	description := m.description

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		body := api.CreateNewAliasJSONRequestBody{
			Domain: domain,
			Format: &format,
		}
		if d := strings.TrimSpace(description); d != "" {
			body.Description = &d
		}
		if len(m.selected) > 0 {
			ids := make([]string, 0, len(m.selected))
			for _, r := range m.recipients {
				if m.selected[r.id] {
					ids = append(ids, r.id)
				}
			}
			body.RecipientIds = &ids
		}

		resp, err := m.client.CreateNewAliasWithResponse(ctx, &api.CreateNewAliasParams{}, body)
		if err != nil {
			return createDoneMsg{err: fmt.Errorf("request failed: %w", err)}
		}
		if resp.JSON201 == nil {
			return createDoneMsg{err: fmt.Errorf("unexpected response: %s", resp.Status())}
		}
		return createDoneMsg{}
	}
}

// fetchDomains returns a command that loads the domains available for
// new aliases together with the account's defaults.
func (m CreateModel) fetchDomains() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.GetDomainOptionsWithResponse(ctx, &api.GetDomainOptionsParams{})
		if err != nil {
			return createOptionsMsg{err: fmt.Errorf("request failed: %w", err)}
		}
		if resp.JSON200 == nil {
			return createOptionsMsg{err: fmt.Errorf("unexpected response: %s", resp.Status())}
		}
		msg := createOptionsMsg{}
		if data := resp.JSON200.Data; data != nil {
			msg.domains = *data
		}
		msg.defaultDomain = str(resp.JSON200.DefaultAliasDomain)
		msg.defaultFormat = str(resp.JSON200.DefaultAliasFormat)
		return msg
	}
}
