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

// This file implements the TUI screen with the recipient list, plus the
// recipient type shared with the other recipient screens.

// recipient is an email address that can receive forwarded mail.
type recipient struct {
	id    string
	email string

	description     string
	active          bool
	emailVerifiedAt string // Empty when not verified.
	aliasesCount    int

	canReplySend        bool
	fingerprint         string
	shouldEncrypt       bool
	inlineEncryption    bool
	protectedHeaders    bool
	removePgpKeys       bool
	removePgpSignatures bool

	createdAt string
	updatedAt string
}

// verifiedFilter selects which recipients are listed based on their
// verification state.
type verifiedFilter int

const (
	verifyAll        verifiedFilter = iota
	verifyVerified                  // Verified only.
	verifyUnverified                // Unverified only.
)

// label returns the display name of the verified filter.
func (f verifiedFilter) label() string {
	switch f {
	case verifyAll:
		return "all"
	case verifyVerified:
		return "verified"
	case verifyUnverified:
		return "unverified"
	default:
		return fmt.Sprintf("unknown verified filter %d", f)
	}
}

// recipientsListLoadedMsg carries the result of the recipients fetch
// back to the recipient list model.
type recipientsListLoadedMsg struct {
	recipients []recipient
	err        error
}

// recipientActionDoneMsg carries the result of a mutating API call made
// from the recipient list screen (create, delete).
type recipientActionDoneMsg struct {
	op  string
	err error
}

// recipientsListRefreshMsg asks the recipient list to refetch. The
// details screen sends it when popping back after it changed the
// recipient.
type recipientsListRefreshMsg struct{}

// RecipientListModel is the recipient list screen.
type RecipientListModel struct {
	client     *api.ClientWithResponses
	recipients []recipient
	table      table.Model
	loading    bool // Currently fetching the recipient list?
	err        error
	status     string // Result of the last action.

	// Both are 0 until the first WindowSizeMsg.
	termHeight int
	termWidth  int

	verifiedFilter verifiedFilter

	// Creating a recipient opens an email input.
	creating   bool
	emailInput textinput.Model

	// The user must hit the delete key twice in a row to delete a
	// recipient.
	deleteArm  bool   // Has the user pressed the delete key once?
	deleteName string // Recipient email shown in the confirmation prompt.
}

// updateTableSize resizes the table to fit the terminal, accounting for
// the extra lines for the recipient count and status.
func (m *RecipientListModel) updateTableSize() {
	chromeLines := 3 // Blank line, recipient count or email input, status.
	m.table.SetHeight(max(m.termHeight-chromeLines, 3))
	m.table.SetWidth(max(m.termWidth, 20))
}

// NewRecipientListModel builds the recipient list screen.
func NewRecipientListModel(client *api.ClientWithResponses) RecipientListModel {
	emailInput := textinput.New()
	emailInput.Placeholder = "recipient@example.com"
	emailInput.Prompt = "email: "
	emailInput.CharLimit = 200
	// bubbles v2.2.1 bug: textinput does not display .Placeholder unless .width is non-zero.
	// See: https://github.com/charmbracelet/bubbles/issues/779
	emailInput.SetWidth(20)

	km := table.DefaultKeyMap()
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "VER", Width: 3},
			{Title: "ACT", Width: 3},
			{Title: "EMAIL", Width: 38},
			{Title: "DESCRIPTION", Width: 30},
			{Title: "ALIASES", Width: 7},
		}),
		table.WithKeyMap(km),
		table.WithStyles(tableStyles()),
		table.WithFocused(true),
		table.WithHeight(defaultTableHeight),
	)

	return RecipientListModel{
		client:     client,
		loading:    true,
		table:      t,
		emailInput: emailInput,
	}
}

func (m RecipientListModel) Init() tea.Cmd {
	// Kick off the first fetch.
	return m.fetchRecipients()
}

func (m RecipientListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Any key other than "x" cancels a pending delete confirmation,
		// so deleting requires two "x" presses in a row.
		if m.creating || msg.String() != "x" {
			m.deleteArm = false
		}

		// While the email input is open it consumes all the keys.
		if m.creating {
			switch msg.String() {
			case "enter":
				email := strings.TrimSpace(m.emailInput.Value())
				m.creating = false
				m.emailInput.Blur()
				m.emailInput.SetValue("")
				if email == "" {
					return m, nil
				}
				return m, m.createRecipient(email)
			case "esc":
				m.creating = false
				m.emailInput.Blur()
				m.emailInput.SetValue("")
				m.updateTableSize()
				return m, nil
			}
			var cmd tea.Cmd
			m.emailInput, cmd = m.emailInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		// Quit TUI.
		case "q":
			return m, tea.Quit
		// Go to the previous view.
		case "esc":
			return m, PopView()
		// Open recipient details.
		case "enter":
			if _, ok := m.selected(); ok {
				return m, SwitchViewWith(ViewRecipientDetails,
					recipientSelectedMsg{recipients: m.recipients, index: m.table.Cursor()})
			}
		// Display TUI help.
		case "f1":
			return m, SwitchView(ViewHelp)
		// Create a new recipient.
		case "c":
			m.creating = true
			m.emailInput.Focus()
			m.emailInput.CursorEnd()
			m.updateTableSize()
			return m, nil
		// Delete a recipient (twice).
		case "x":
			r, ok := m.selected()
			if !ok {
				m.deleteArm = false
				return m, nil
			}
			if !m.deleteArm {
				m.deleteArm = true
				m.deleteName = r.email
				return m, nil
			}
			m.deleteArm = false
			return m, m.deleteRecipient(r)
		// Cycle verified filter.
		case "f":
			m.verifiedFilter = (m.verifiedFilter + 1) % 3
			return m, m.fetchRecipients()
		// Refetch recipients.
		case "r":
			return m, m.fetchRecipients()
		}
		// The rest is consumed by the table...
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.termHeight = msg.Height
		m.termWidth = msg.Width
		m.updateTableSize()

	case recipientsListLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.deleteArm = false
		m.recipients = msg.recipients
		m.table.SetRows(recipientRows(msg.recipients))

	case recipientsListRefreshMsg:
		return m, m.fetchRecipients()

	case recipientActionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %s", msg.op, describeError(msg.err))
			return m, nil
		}
		m.status = msg.op
		return m, m.fetchRecipients() // Refetch after actions.
	}
	return m, nil
}

// selected returns the recipient under the table cursor, if any.
func (m RecipientListModel) selected() (recipient, bool) {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.recipients) {
		return recipient{}, false
	}
	return m.recipients[i], true
}

func (m RecipientListModel) View() tea.View {
	var b strings.Builder

	switch {
	case m.err != nil:
		fmt.Fprintf(&b, "Error: %s\n\n", describeError(m.err))
	case m.loading:
		b.WriteString("Loading recipients...\n")
	case len(m.recipients) == 0:
		b.WriteString("No recipients found.\n")
	default:
		b.WriteString(m.table.View())
	}

	if m.creating {
		fmt.Fprintf(&b, "\n\n%s", m.emailInput.View())
	} else {
		fmt.Fprintf(&b, "\n\n%d/%d recipients", m.table.Cursor()+1, len(m.recipients))
	}

	if m.deleteArm {
		// The pending delete confirmation replaces the whole status
		// line so it can't be missed.
		fmt.Fprintf(&b, "\npress x again to DELETE %s permanently (cannot be undone)", m.deleteName)
	} else {
		fmt.Fprintf(&b, "\nfilter: %s", m.verifiedFilter.label())
		if m.status != "" {
			fmt.Fprintf(&b, " - %s", m.status)
		}
	}

	return tea.NewView(b.String())
}

// recipientRows converts recipients into table rows.
func recipientRows(recipients []recipient) []table.Row {
	rows := make([]table.Row, 0, len(recipients))
	for _, r := range recipients {
		ver, act := " ", " "
		if r.emailVerifiedAt != "" {
			ver = "+"
		}
		if r.active {
			act = "+"
		}
		description := r.description
		if description == "" {
			description = "-"
		}
		rows = append(rows, table.Row{
			ver,
			act,
			r.email,
			description,
			fmt.Sprintf("%d", r.aliasesCount),
		})
	}
	return rows
}

// fetchRecipients fetches the recipients using the model's current
// verified filter.
func (m RecipientListModel) fetchRecipients() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// The alias counts power the ALIASES column.
		withAliasCount := "true"
		params := &api.GetAllRecipientsParams{
			FilterAliasCount: &withAliasCount,
		}

		switch m.verifiedFilter {
		case verifyVerified:
			v := "true"
			params.FilterVerified = &v
		case verifyUnverified:
			v := "false"
			params.FilterVerified = &v
		}
		// verifyAll is the API default, thus no handler.

		resp, err := m.client.GetAllRecipientsWithResponse(ctx, params)
		if err != nil {
			return recipientsListLoadedMsg{err: fmt.Errorf("request failed: %w", err)}
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			return recipientsListLoadedMsg{err: fmt.Errorf("unexpected response: %s", resp.Status())}
		}
		msg := recipientsListLoadedMsg{}
		for _, r := range *resp.JSON200.Data {
			msg.recipients = append(msg.recipients, recipient{
				id:              str(r.Id),
				email:           str(r.Email),
				description:     str(r.Description),
				active:          r.Active != nil && *r.Active,
				emailVerifiedAt: str(r.EmailVerifiedAt),
				aliasesCount:    intVal(r.AliasesCount),
			})
		}
		return msg
	}
}

// createRecipient adds a new recipient email address.
func (m RecipientListModel) createRecipient(email string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.CreateNewRecipientWithResponse(ctx,
			&api.CreateNewRecipientParams{},
			api.CreateNewRecipientJSONRequestBody{Email: email})
		if err := apiError(err, resp); err != nil {
			return recipientActionDoneMsg{op: "create recipient", err: err}
		}
		return recipientActionDoneMsg{op: "created recipient"}
	}
}

// deleteRecipient permanently deletes the recipient.
func (m RecipientListModel) deleteRecipient(r recipient) tea.Cmd {
	if r.id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.DeleteASpecificRecipientWithResponse(ctx, r.id,
			&api.DeleteASpecificRecipientParams{})
		if err := apiError(err, resp); err != nil {
			return recipientActionDoneMsg{op: "delete recipient", err: err}
		}
		return recipientActionDoneMsg{op: "deleted recipient"}
	}
}
