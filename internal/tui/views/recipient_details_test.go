package views

import (
	"context"
	"testing"

	"github.com/kovmir/addyui/internal/mockaddy"
)

// fullTestRecipient covers every field recipientDetailsRows renders.
func fullTestRecipient() recipient {
	return recipient{
		id:                  "46eebc50-f7f8-46d7-beb9-c37f04c29a84",
		email:               "me@example.com",
		description:         "personal",
		active:              true,
		emailVerifiedAt:     "2019-10-01 09:00:00",
		aliasesCount:        5,
		canReplySend:        true,
		fingerprint:         "ABCD1234",
		shouldEncrypt:       true,
		inlineEncryption:    true,
		protectedHeaders:    true,
		removePgpKeys:       true,
		removePgpSignatures: true,
		createdAt:           "2019-10-01 09:00:00",
		updatedAt:           "2020-11-11 11:11:11",
	}
}

func TestRecipientDetailRowCountMatchesConstants(t *testing.T) {
	t.Parallel()

	// The row indices (recipientRowXxx) must stay in sync with
	// recipientDetailsRows.
	rows := recipientDetailsRows(&recipient{})
	if len(rows) != recipientRowUpdated+1 {
		t.Errorf("recipientDetailsRows has %d rows, but recipientRowUpdated = %d",
			len(rows), recipientRowUpdated)
	}
	if got := recipientDetailRowCount(); got != len(rows) {
		t.Errorf("recipientDetailRowCount() = %d, want %d", got, len(rows))
	}
}

func TestRecipientDetailsRowsValues(t *testing.T) {
	t.Parallel()

	r := fullTestRecipient()
	rows := recipientDetailsRows(&r)

	checks := []struct {
		row  int
		want string
	}{
		{recipientRowEmail, "me@example.com"},
		{recipientRowDescription, "personal"},
		{recipientRowActive, "true"},
		{recipientRowVerified, "2019-10-01 09:00:00"},
		{recipientRowAliases, "5"},
		{recipientRowCanReplySend, "true"},
		{recipientRowEncryption, "true"},
		{recipientRowInlineEncryption, "true"},
		{recipientRowProtectedHeaders, "true"},
		{recipientRowRemovePgpKeys, "true"},
		{recipientRowRemovePgpSignatures, "true"},
		{recipientRowPublicKey, "present"},
		{recipientRowRemoveKey, "press space to remove"},
		{recipientRowFingerprint, "ABCD1234"},
		{recipientRowCreated, "2019-10-01 09:00:00"},
		{recipientRowUpdated, "2020-11-11 11:11:11"},
	}
	for _, c := range checks {
		if rows[c.row][1] != c.want {
			t.Errorf("row %d value = %q, want %q", c.row, rows[c.row][1], c.want)
		}
	}
}

func TestRecipientDetailsRowsEmptyRecipient(t *testing.T) {
	t.Parallel()

	// A bare recipient must render placeholders, not panic.
	rows := recipientDetailsRows(&recipient{})

	checks := []struct {
		row  int
		want string
	}{
		{recipientRowEmail, ""},
		{recipientRowDescription, "-"},
		{recipientRowActive, "false"},
		{recipientRowVerified, "not verified"},
		{recipientRowAliases, "0"},
		{recipientRowPublicKey, "not set"},
		{recipientRowRemoveKey, "-"},
		{recipientRowFingerprint, "-"},
	}
	for _, c := range checks {
		if rows[c.row][1] != c.want {
			t.Errorf("row %d value = %q, want %q", c.row, rows[c.row][1], c.want)
		}
	}
}

func detailsRecipientModel(t *testing.T, opts ...mockaddy.Option) RecipientDetailsModel {
	t.Helper()
	client := newTestClient(t, opts...)
	m := NewRecipientDetailsModel(client)
	recipients := []recipient{fullTestRecipient()}
	// The server must know the recipient too, so commands succeed.
	m.recipients = recipients
	m.index = 0
	return m
}

func TestRecipientDetailsRefreshKeepsActive(t *testing.T) {
	t.Parallel()

	// The API's single-recipient endpoint does not return the active
	// flag, so the refresh must keep the snapshot's value instead of
	// resetting it to false.
	id := fullTestRecipient().id
	m := detailsRecipientModel(t,
		mockaddy.WithRecipients([]mockaddy.Recipient{{
			ID: id, Email: "me@example.com", Active: true,
			EmailVerifiedAt: new("2019-10-01 09:00:00"),
			CreatedAt:       "2019-10-01 09:00:00", UpdatedAt: "2020-11-11 11:11:11",
		}}))

	msg := m.refreshMsg(context.Background(), id, "refreshed")
	done, ok := msg.(recipientDetailsActionDoneMsg)
	if !ok {
		t.Fatalf("refresh produced %T, want recipientDetailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("refresh failed: %v", done.err)
	}
	if done.recipient == nil {
		t.Fatal("refresh must carry the refreshed recipient")
	}
	if !done.recipient.active {
		t.Error("refresh reset the active flag, want it kept from the snapshot")
	}
	if done.recipient.emailVerifiedAt != "2019-10-01 09:00:00" {
		t.Errorf("emailVerifiedAt = %q, want the server value", done.recipient.emailVerifiedAt)
	}
}

func TestRecipientDetailsToggleActive(t *testing.T) {
	t.Parallel()

	id := fullTestRecipient().id
	client := newTestClient(t,
		mockaddy.WithRecipients([]mockaddy.Recipient{
			{ID: id, Email: "me@example.com", Active: true,
				CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
		}))
	m := NewRecipientDetailsModel(client)
	r := fullTestRecipient()
	m.recipients = []recipient{r}
	m.index = 0

	msg := m.toggleActive(&m.recipients[0])() // run the tea.Cmd
	done, ok := msg.(recipientDetailsActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientDetailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("toggle active failed: %v", done.err)
	}
	if done.op != "toggled active state" {
		t.Errorf("op = %q, want \"toggled active state\"", done.op)
	}
}

func TestRecipientDetailsToggleActiveErrorSurfaces(t *testing.T) {
	t.Parallel()

	// The recipient does not exist on the fake server, so the toggle
	// must come back as an error rather than a success.
	id := "does-not-exist"
	m := detailsRecipientModel(t)
	r := fullTestRecipient()
	r.id = id
	m.recipients = []recipient{r}
	m.index = 0

	msg := m.toggleActive(&m.recipients[0])() // run the tea.Cmd
	done, ok := msg.(recipientDetailsActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientDetailsActionDoneMsg", msg)
	}
	if done.err == nil {
		t.Error("toggle of a missing recipient must produce an error")
	}
	if done.recipient != nil {
		t.Errorf("a failed toggle must not carry a recipient: %+v", done.recipient)
	}
}

func TestRecipientDetailsResendVerification(t *testing.T) {
	t.Parallel()

	id := fullTestRecipient().id
	m := detailsRecipientModel(t,
		mockaddy.WithRecipients([]mockaddy.Recipient{
			{ID: id, Email: "me@example.com", CreatedAt: "2026-01-01 00:00:00"},
		}))

	msg := m.resendVerification(&m.recipients[0])() // run the tea.Cmd
	done, ok := msg.(recipientDetailsActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientDetailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("resend verification failed: %v", done.err)
	}
	if done.op != "verification email sent" {
		t.Errorf("op = %q, want \"verification email sent\"", done.op)
	}
}

func TestRecipientDetailsAddAndRemoveKey(t *testing.T) {
	t.Parallel()

	id := fullTestRecipient().id
	m := detailsRecipientModel(t,
		mockaddy.WithRecipients([]mockaddy.Recipient{
			{ID: id, Email: "me@example.com", CreatedAt: "2026-01-01 00:00:00"},
		}))

	// Add a key: the refreshed snapshot carries the mock fingerprint.
	msg := m.addPublicKey("-----BEGIN PGP PUBLIC KEY BLOCK-----")()
	done, ok := msg.(recipientDetailsActionDoneMsg)
	if !ok {
		t.Fatalf("add key produced %T, want recipientDetailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("add key failed: %v", done.err)
	}
	if done.recipient == nil || done.recipient.fingerprint == "" {
		t.Error("add key must set the fingerprint on the refreshed recipient")
	}

	// Remove it again: the fingerprint is gone.
	r := fullTestRecipient()
	msg = m.removeKey(&r)()
	done, ok = msg.(recipientDetailsActionDoneMsg)
	if !ok {
		t.Fatalf("remove key produced %T, want recipientDetailsActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("remove key failed: %v", done.err)
	}
	if done.recipient == nil || done.recipient.fingerprint != "" {
		t.Error("remove key must clear the fingerprint on the refreshed recipient")
	}
}

func TestRecipientDetailsRemoveKeyNoopWithoutKey(t *testing.T) {
	t.Parallel()

	// A recipient without a key must not trigger a request.
	r := fullTestRecipient()
	r.fingerprint = ""
	m := detailsRecipientModel(t)
	if cmd := m.removeKey(&r); cmd != nil {
		t.Error("removeKey without a fingerprint must return nil")
	}
}

func TestAliasListForRecipientMsg(t *testing.T) {
	t.Parallel()

	client := newTestClient(t,
		mockaddy.WithRecipients(seedRecipients()),
		mockaddy.WithAliases(seedRecipientAliases()))

	m := NewListModel(client, "", StateActiveInactive)
	updated, cmd := m.Update(listForRecipientMsg{recipientID: "r1", email: "me@example.com"})
	if cmd != nil {
		t.Fatal("listForRecipientMsg must not return a command; Init() fetches")
	}
	lm := updated.(ListModel)
	if lm.recipientFilter != "r1" || lm.recipientEmail != "me@example.com" {
		t.Errorf("filter = %q/%q, want r1/me@example.com",
			lm.recipientFilter, lm.recipientEmail)
	}
	if lm.page != 1 || lm.table.Cursor() != 0 {
		t.Error("listForRecipientMsg must reset the page and cursor")
	}

	// The filtered fetch must only return aliases attached to r1.
	loaded := lm.fetchAliases()().(aliasesLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("filtered fetch failed: %v", loaded.err)
	}
	if len(loaded.aliases) != 2 {
		t.Fatalf("filtered fetch returned %d aliases, want 2 (a1 and a2)", len(loaded.aliases))
	}
	for _, a := range loaded.aliases {
		found := false
		for _, r := range *a.Recipients {
			if obj, ok := r.(map[string]interface{}); ok && obj["id"] == "r1" {
				found = true
			}
		}
		if !found {
			t.Errorf("alias %s is not attached to r1", str(a.Email))
		}
	}
}
