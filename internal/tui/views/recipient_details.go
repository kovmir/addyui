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

// This file implements the TUI screen with the recipient details.

// Row indices of the recipient details table fields; they match the
// table rows built by recipientDetailsRows.
const (
	recipientRowEmail = iota
	recipientRowDescription
	recipientRowActive
	recipientRowVerified
	recipientRowAliases
	recipientRowCanReplySend
	recipientRowEncryption
	recipientRowInlineEncryption
	recipientRowProtectedHeaders
	recipientRowRemovePgpKeys
	recipientRowRemovePgpSignatures
	recipientRowPublicKey
	recipientRowRemoveKey
	recipientRowFingerprint
	recipientRowCreated
	recipientRowUpdated
)

// recipientSelectedMsg carries the recipient list snapshot and the
// selected index from the recipient list screen to the details screen.
type recipientSelectedMsg struct {
	recipients []recipient
	index      int
}

// recipientDetailsActionDoneMsg carries the result of a mutating API
// call made from the recipient details screen, plus the refreshed
// recipient on success so the snapshot stays in sync with the server.
type recipientDetailsActionDoneMsg struct {
	op        string
	recipient *recipient // fresh recipient from the API; nil on error
	err       error
}

// RecipientDetailsModel shows the attributes of a single recipient as a
// two-column table.
type RecipientDetailsModel struct {
	client     *api.ClientWithResponses
	recipients []recipient // Snapshot of the list screen.
	index      int         // Position within recipients.
	table      table.Model
	status     string // Result of the last action.
	dirty      bool   // An action succeeded; recipient list should refetch.

	// Description text editor.
	descEditor    textinput.Model
	descIsEditing bool

	// PGP public key paste editor.
	keyEditor    textinput.Model
	keyIsEditing bool
}

func NewRecipientDetailsModel(client *api.ClientWithResponses) RecipientDetailsModel {
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

	// An armored public key easily exceeds a thousand characters.
	key := textinput.New()
	key.Placeholder = "-----BEGIN PGP PUBLIC KEY BLOCK-----"
	key.Prompt = "key: "
	key.CharLimit = 8192
	key.SetWidth(20)

	return RecipientDetailsModel{
		client:     client,
		table:      t,
		descEditor: input,
		keyEditor:  key,
	}
}

func (m RecipientDetailsModel) Init() tea.Cmd {
	return nil
}

func (m RecipientDetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		// Let the value column use the rest of the terminal width.
		m.table.SetColumns([]table.Column{
			{Title: "PROPERTY", Width: fieldColWidth},
			{Title: "VALUE", Width: max(msg.Width-fieldColWidth, 20)},
		})
		// Show all rows without scrolling when they fit.
		m.table.SetHeight(min(recipientDetailRowCount()+1, max(msg.Height-1, 3)))
		m.descEditor.SetWidth(max(msg.Width-2, 20))
		m.keyEditor.SetWidth(max(msg.Width-2, 20))

	case recipientSelectedMsg:
		m.recipients = msg.recipients
		m.index = msg.index
		// A fresh entry: drop any transient state from the last visit.
		m.descIsEditing = false
		m.keyIsEditing = false
		m.status = ""
		m.dirty = false
		// Rows must be set before the cursor: SetCursor clamps against
		// the current row count, so with no rows it would yield -1 and
		// neither the highlight nor the last rows would render.
		m.table.SetRows(recipientDetailsRows(&m.recipients[m.index]))
		m.table.SetCursor(0)

	case recipientDetailsActionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %s", msg.op, describeError(msg.err))
			return m, nil
		}
		m.status = msg.op
		m.dirty = true
		if msg.recipient != nil {
			m.recipients[m.index] = *msg.recipient
		}
		m.syncRows()

	case tea.KeyPressMsg:
		// While the key editor is open it consumes all keys, except the
		// two that leave it.
		if m.keyIsEditing {
			switch msg.String() {
			case "enter": // add or replace the public key
				m.keyIsEditing = false
				m.keyEditor.Blur()
				return m, m.addPublicKey(strings.TrimSpace(m.keyEditor.Value()))
			case "esc": // cancel, keep the old key
				m.keyIsEditing = false
				m.keyEditor.Blur()
				m.keyEditor.SetValue("")
				return m, nil
			}
			var cmd tea.Cmd
			m.keyEditor, cmd = m.keyEditor.Update(msg)
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
				return m, PopViewWith(recipientsListRefreshMsg{})
			}
			return m, PopView()
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

// move steps to the neighboring recipient, clamped to the list bounds.
func (m *RecipientDetailsModel) move(delta int) {
	if len(m.recipients) == 0 {
		return
	}
	m.index = min(max(m.index+delta, 0), len(m.recipients)-1)
	m.table.SetCursor(0)
	m.syncRows()
}

// recipientInteractiveRowIndices lists the row indices space can act
// on, in display order; they match the rows handled by interact.
var recipientInteractiveRowIndices = []int{
	recipientRowDescription,
	recipientRowActive,
	recipientRowVerified,
	recipientRowAliases,
	recipientRowCanReplySend,
	recipientRowEncryption,
	recipientRowInlineEncryption,
	recipientRowProtectedHeaders,
	recipientRowRemovePgpKeys,
	recipientRowRemovePgpSignatures,
	recipientRowPublicKey,
	recipientRowRemoveKey,
}

// moveCursorInteractive moves the table cursor to the next interactive
// row, or to the previous one for a negative delta. It clamps at the
// first and last interactive rows, so the cursor never rests on a row
// space cannot act on.
func (m *RecipientDetailsModel) moveCursorInteractive(delta int) {
	cur := m.table.Cursor()
	next := cur
	if delta > 0 {
		for _, row := range recipientInteractiveRowIndices {
			if row > cur {
				next = row
				break
			}
		}
	} else {
		for i := len(recipientInteractiveRowIndices) - 1; i >= 0; i-- {
			if row := recipientInteractiveRowIndices[i]; row < cur {
				next = row
				break
			}
		}
	}
	m.table.SetCursor(next)
}

// interact starts the interaction of the row under the cursor: editing
// for the description and the public key, a view switch for the alias
// list, and an API toggle for everything else. Non-interactive rows
// (email, fingerprint, timestamps) do nothing.
func (m *RecipientDetailsModel) interact(row int) tea.Cmd {
	if m.index < 0 || m.index >= len(m.recipients) {
		return nil
	}
	r := &m.recipients[m.index]
	switch row {
	case recipientRowDescription:
		m.descIsEditing = true
		m.descEditor.SetValue(r.description)
		cmd := m.descEditor.Focus()
		m.descEditor.CursorEnd()
		return cmd
	case recipientRowActive:
		return m.toggleActive(r)
	case recipientRowVerified:
		return m.resendVerification(r)
	case recipientRowAliases:
		// Open the alias list restricted to this recipient.
		return SwitchViewWith(ViewRecipientAliases, listForRecipientMsg{
			recipientID: r.id,
			email:       r.email,
		})
	case recipientRowCanReplySend:
		return m.toggleReplySend(r)
	case recipientRowEncryption:
		return m.toggleEncryption(r)
	case recipientRowInlineEncryption:
		return m.toggleInlineEncryption(r)
	case recipientRowProtectedHeaders:
		return m.toggleProtectedHeaders(r)
	case recipientRowRemovePgpKeys:
		return m.toggleRemovePgpKeys(r)
	case recipientRowRemovePgpSignatures:
		return m.toggleRemovePgpSignatures(r)
	case recipientRowPublicKey:
		m.keyIsEditing = true
		cmd := m.keyEditor.Focus()
		m.keyEditor.CursorEnd()
		return cmd
	case recipientRowRemoveKey:
		return m.removeKey(r)
	}
	return nil
}

// updateDescription sends the new description to the API, then
// refreshes the snapshot. Sending nothing when the value is unchanged
// avoids a pointless request.
func (m *RecipientDetailsModel) updateDescription(description string) tea.Cmd {
	if m.index < 0 || m.index >= len(m.recipients) {
		return nil
	}
	r := &m.recipients[m.index]
	if r.id == "" || description == r.description {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.UpdateASpecificRecipientWithResponse(ctx, r.id,
			&api.UpdateASpecificRecipientParams{},
			api.UpdateASpecificRecipientJSONRequestBody{Description: &description})
		if err = apiError(err, resp); err != nil {
			return recipientDetailsActionDoneMsg{op: "update description", err: err}
		}
		return m.refreshMsg(ctx, r.id, "updated description")
	}
}

// toggleActive activates the recipient if it is inactive, and vice
// versa.
func (m *RecipientDetailsModel) toggleActive(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	activate := !r.active
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if activate {
			var resp *api.ActivateASpecificRecipientResponse
			resp, err = m.client.ActivateASpecificRecipientWithResponse(ctx,
				&api.ActivateASpecificRecipientParams{},
				api.ActivateASpecificRecipientJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DeactivateASpecificRecipientResponse
			resp, err = m.client.DeactivateASpecificRecipientWithResponse(ctx, id,
				&api.DeactivateASpecificRecipientParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle active", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled active state")
	}
}

// resendVerification asks addy.io to resend the verification email.
func (m *RecipientDetailsModel) resendVerification(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.ResendTheVerificationEmailForARecipientWithResponse(ctx,
			&api.ResendTheVerificationEmailForARecipientParams{},
			api.ResendTheVerificationEmailForARecipientJSONRequestBody{RecipientId: id})
		if err = apiError(err, resp); err != nil {
			return recipientDetailsActionDoneMsg{op: "resend verification", err: err}
		}
		return m.refreshMsg(ctx, id, "verification email sent")
	}
}

// toggleReplySend allows the recipient to reply/send from the user's
// aliases if disallowed, and vice versa.
func (m *RecipientDetailsModel) toggleReplySend(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	allow := !r.canReplySend
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if allow {
			var resp *api.AllowASpecificRecipientToReplysendFromYourAliasesResponse
			resp, err = m.client.AllowASpecificRecipientToReplysendFromYourAliasesWithResponse(ctx,
				&api.AllowASpecificRecipientToReplysendFromYourAliasesParams{},
				api.AllowASpecificRecipientToReplysendFromYourAliasesJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DisallowASpecificRecipientToReplysendFromYourAliasesResponse
			resp, err = m.client.DisallowASpecificRecipientToReplysendFromYourAliasesWithResponse(ctx, id,
				&api.DisallowASpecificRecipientToReplysendFromYourAliasesParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle reply/send", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled reply/send")
	}
}

// toggleEncryption turns PGP encryption for the recipient on or off.
func (m *RecipientDetailsModel) toggleEncryption(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	enable := !r.shouldEncrypt
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if enable {
			var resp *api.EnableEncryptionForASpecificRecipientResponse
			resp, err = m.client.EnableEncryptionForASpecificRecipientWithResponse(ctx,
				&api.EnableEncryptionForASpecificRecipientParams{},
				api.EnableEncryptionForASpecificRecipientJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DisableEncryptionForASpecificRecipientResponse
			resp, err = m.client.DisableEncryptionForASpecificRecipientWithResponse(ctx, id,
				&api.DisableEncryptionForASpecificRecipientParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle encryption", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled encryption")
	}
}

// toggleInlineEncryption turns inline PGP encryption on or off.
func (m *RecipientDetailsModel) toggleInlineEncryption(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	enable := !r.inlineEncryption
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if enable {
			var resp *api.EnableInlinePGPInlineEncryptionForASpecificRecipientResponse
			resp, err = m.client.EnableInlinePGPInlineEncryptionForASpecificRecipientWithResponse(ctx,
				&api.EnableInlinePGPInlineEncryptionForASpecificRecipientParams{},
				api.EnableInlinePGPInlineEncryptionForASpecificRecipientJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DisableInlinePGPInlineEncryptionForASpecificRecipientResponse
			resp, err = m.client.DisableInlinePGPInlineEncryptionForASpecificRecipientWithResponse(ctx, id,
				&api.DisableInlinePGPInlineEncryptionForASpecificRecipientParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle inline encryption", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled inline encryption")
	}
}

// toggleProtectedHeaders turns hiding the subject in protected headers
// on or off.
func (m *RecipientDetailsModel) toggleProtectedHeaders(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	enable := !r.protectedHeaders
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if enable {
			var resp *api.EnableProtectedHeadershideSubjectForASpecificRecipientResponse
			resp, err = m.client.EnableProtectedHeadershideSubjectForASpecificRecipientWithResponse(ctx,
				&api.EnableProtectedHeadershideSubjectForASpecificRecipientParams{},
				api.EnableProtectedHeadershideSubjectForASpecificRecipientJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DisableProtectedHeadershideSubjectForASpecificRecipientResponse
			resp, err = m.client.DisableProtectedHeadershideSubjectForASpecificRecipientWithResponse(ctx, id,
				&api.DisableProtectedHeadershideSubjectForASpecificRecipientParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle protected headers", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled protected headers")
	}
}

// toggleRemovePgpKeys turns stripping PGP keys from outgoing mail on or
// off.
func (m *RecipientDetailsModel) toggleRemovePgpKeys(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	enable := !r.removePgpKeys
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if enable {
			var resp *api.EnableRemovePGPKeysForASpecificRecipientResponse
			resp, err = m.client.EnableRemovePGPKeysForASpecificRecipientWithResponse(ctx,
				&api.EnableRemovePGPKeysForASpecificRecipientParams{},
				api.EnableRemovePGPKeysForASpecificRecipientJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DisableRemovePGPKeysForASpecificRecipientResponse
			resp, err = m.client.DisableRemovePGPKeysForASpecificRecipientWithResponse(ctx, id,
				&api.DisableRemovePGPKeysForASpecificRecipientParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle remove PGP keys", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled remove PGP keys")
	}
}

// toggleRemovePgpSignatures turns stripping PGP signatures from
// outgoing mail on or off.
func (m *RecipientDetailsModel) toggleRemovePgpSignatures(r *recipient) tea.Cmd {
	id := r.id
	if id == "" {
		return nil
	}
	enable := !r.removePgpSignatures
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var err error
		if enable {
			var resp *api.EnableRemovePGPSignaturesForASpecificRecipientResponse
			resp, err = m.client.EnableRemovePGPSignaturesForASpecificRecipientWithResponse(ctx,
				&api.EnableRemovePGPSignaturesForASpecificRecipientParams{},
				api.EnableRemovePGPSignaturesForASpecificRecipientJSONRequestBody{Id: id})
			err = apiError(err, resp)
		} else {
			var resp *api.DisableRemovePGPSignaturesForASpecificRecipientResponse
			resp, err = m.client.DisableRemovePGPSignaturesForASpecificRecipientWithResponse(ctx, id,
				&api.DisableRemovePGPSignaturesForASpecificRecipientParams{})
			err = apiError(err, resp)
		}
		if err != nil {
			return recipientDetailsActionDoneMsg{op: "toggle remove PGP signatures", err: err}
		}
		return m.refreshMsg(ctx, id, "toggled remove PGP signatures")
	}
}

// addPublicKey replaces the recipient's PGP public key.
func (m *RecipientDetailsModel) addPublicKey(keyData string) tea.Cmd {
	if m.index < 0 || m.index >= len(m.recipients) {
		return nil
	}
	id := m.recipients[m.index].id
	if id == "" || keyData == "" {
		m.keyEditor.SetValue("")
		return nil
	}
	m.keyEditor.SetValue("")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.AddPublicKeyForASpecificRecipientWithResponse(ctx, id,
			&api.AddPublicKeyForASpecificRecipientParams{},
			api.AddPublicKeyForASpecificRecipientJSONRequestBody{KeyData: keyData})
		if err = apiError(err, resp); err != nil {
			return recipientDetailsActionDoneMsg{op: "add public key", err: err}
		}
		return m.refreshMsg(ctx, id, "public key added")
	}
}

// removeKey deletes the recipient's PGP public key.
func (m *RecipientDetailsModel) removeKey(r *recipient) tea.Cmd {
	id := r.id
	if id == "" || r.fingerprint == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := m.client.RemovePublicKeyForASpecificRecipientWithResponse(ctx, id,
			&api.RemovePublicKeyForASpecificRecipientParams{})
		if err = apiError(err, resp); err != nil {
			return recipientDetailsActionDoneMsg{op: "remove public key", err: err}
		}
		return m.refreshMsg(ctx, id, "public key removed")
	}
}

// refreshMsg fetches the recipient and returns a
// recipientDetailsActionDoneMsg carrying the fresh snapshot, so the
// table reflects the server state. The API's single-recipient endpoint
// does not return the active flag, so the snapshot's value is kept.
func (m *RecipientDetailsModel) refreshMsg(ctx context.Context, id, op string) tea.Msg {
	resp, err := m.client.GetASpecificRecipientWithResponse(ctx, id, &api.GetASpecificRecipientParams{})
	if err != nil {
		return recipientDetailsActionDoneMsg{op: op, err: fmt.Errorf("refresh failed: %w", err)}
	}
	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return recipientDetailsActionDoneMsg{op: op, err: fmt.Errorf("refresh failed: %s", resp.Status())}
	}
	data := resp.JSON200.Data
	r := recipient{
		id:                  str(data.Id),
		email:               str(data.Email),
		description:         str(data.Description),
		active:              m.recipients[m.index].active,
		emailVerifiedAt:     str(data.EmailVerifiedAt),
		aliasesCount:        intVal(data.AliasesCount),
		canReplySend:        data.CanReplySend != nil && *data.CanReplySend,
		fingerprint:         str(data.Fingerprint),
		shouldEncrypt:       data.ShouldEncrypt != nil && *data.ShouldEncrypt,
		inlineEncryption:    data.InlineEncryption != nil && *data.InlineEncryption,
		protectedHeaders:    data.ProtectedHeaders != nil && *data.ProtectedHeaders,
		removePgpKeys:       data.RemovePgpKeys != nil && *data.RemovePgpKeys,
		removePgpSignatures: data.RemovePgpSignatures != nil && *data.RemovePgpSignatures,
		createdAt:           str(data.CreatedAt),
		updatedAt:           str(data.UpdatedAt),
	}
	return recipientDetailsActionDoneMsg{op: op, recipient: &r}
}

// syncRows refreshes the table rows after the recipient snapshot
// changed.
func (m *RecipientDetailsModel) syncRows() {
	if m.index < 0 || m.index >= len(m.recipients) {
		return
	}
	m.table.SetRows(recipientDetailsRows(&m.recipients[m.index]))
}

func (m RecipientDetailsModel) View() tea.View {
	var b strings.Builder
	b.WriteString(m.table.View())
	if m.status != "" {
		fmt.Fprintf(&b, " - %s", m.status)
	}

	if m.descIsEditing {
		fmt.Fprintf(&b, "\n\n%s", m.descEditor.View())
	} else if m.keyIsEditing {
		fmt.Fprintf(&b, "\n\n%s", m.keyEditor.View())
	} else {
		if len(m.recipients) > 0 {
			fmt.Fprintf(&b, "\n\n%d/%d recipients", m.index+1, len(m.recipients))
		}
	}
	return tea.NewView(b.String())
}

// recipientDetailRowCount returns the number of rows the recipient
// details table renders.
func recipientDetailRowCount() int {
	return len(recipientDetailsRows(&recipient{}))
}

// recipientDetailsRows converts a recipient into rows for the details
// table. The row order matches the recipientRowXxx constants.
func recipientDetailsRows(r *recipient) []table.Row {
	description := r.description
	if description == "" {
		description = "-"
	}
	verified := r.emailVerifiedAt
	if verified == "" {
		verified = "not verified"
	}
	fingerprint := r.fingerprint
	if fingerprint == "" {
		fingerprint = "-"
	}
	publicKey := "not set"
	removeKey := "-"
	if r.fingerprint != "" {
		publicKey = "present"
		removeKey = "press space to remove"
	}

	return []table.Row{
		{"Email", r.email},
		{"Description", description},
		{"Active", fmt.Sprintf("%v", r.active)},
		{"Verified", verified},
		{"Aliases", fmt.Sprintf("%d", r.aliasesCount)},
		{"Can reply/send", fmt.Sprintf("%v", r.canReplySend)},
		{"Encryption", fmt.Sprintf("%v", r.shouldEncrypt)},
		{"Inline encryption", fmt.Sprintf("%v", r.inlineEncryption)},
		{"Protected headers", fmt.Sprintf("%v", r.protectedHeaders)},
		{"Remove PGP keys", fmt.Sprintf("%v", r.removePgpKeys)},
		{"Remove PGP signatures", fmt.Sprintf("%v", r.removePgpSignatures)},
		{"Public key", publicKey},
		{"Remove key", removeKey},
		{"Fingerprint", fingerprint},
		{"Created", r.createdAt},
		{"Updated", r.updatedAt},
	}
}
