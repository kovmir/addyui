package views

import (
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/kovmir/addyui/internal/mockaddy"
)

func seedRecipientAliases() []mockaddy.Alias {
	verified := "2026-01-01 00:00:00"
	return []mockaddy.Alias{
		{ID: "a1", Email: "one@demo.anonaddy.com", Active: true,
			CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00",
			Recipients: []mockaddy.Recipient{
				{ID: "r1", Email: "me@example.com", EmailVerifiedAt: &verified},
			}},
		{ID: "a2", Email: "two@demo.anonaddy.com", Active: true,
			CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00",
			Recipients: []mockaddy.Recipient{
				{ID: "r1", Email: "me@example.com", EmailVerifiedAt: &verified},
				{ID: "r2", Email: "you@example.com", EmailVerifiedAt: &verified},
			}},
		{ID: "a3", Email: "three@demo.anonaddy.com", Active: true,
			CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
	}
}

func seedRecipients() []mockaddy.Recipient {
	verified := "2026-01-01 00:00:00"
	return []mockaddy.Recipient{
		{ID: "r1", Email: "me@example.com", Active: true,
			EmailVerifiedAt: &verified, CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
		{ID: "r2", Email: "you@example.com", Active: true,
			EmailVerifiedAt: &verified, CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
		{ID: "r3", Email: "pending@example.net", Active: true,
			CreatedAt: "2026-01-01 00:00:00", UpdatedAt: "2026-01-01 00:00:00"},
	}
}

func TestRecipientListFetch(t *testing.T) {
	t.Parallel()

	client := newTestClient(t,
		mockaddy.WithRecipients(seedRecipients()),
		mockaddy.WithAliases(seedRecipientAliases()))

	m := NewRecipientListModel(client)
	msg := m.fetchRecipients()() // run the tea.Cmd
	loaded, ok := msg.(recipientsListLoadedMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientsListLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("recipientsListLoadedMsg error = %v", loaded.err)
	}
	if len(loaded.recipients) != 3 {
		t.Fatalf("recipients = %d entries, want 3", len(loaded.recipients))
	}

	r := loaded.recipients[0]
	if r.id != "r1" || r.email != "me@example.com" {
		t.Errorf("recipients[0] = %+v, want {r1 me@example.com}", r)
	}
	if !r.active {
		t.Error("recipients[0].active = false, want true")
	}
	if r.emailVerifiedAt == "" {
		t.Error("recipients[0].emailVerifiedAt is empty, want the timestamp")
	}
	if r.aliasesCount != 2 {
		t.Errorf("recipients[0].aliasesCount = %d, want 2 (aliases a1 and a2)", r.aliasesCount)
	}
	if loaded.recipients[2].emailVerifiedAt != "" {
		t.Error("recipients[2].emailVerifiedAt is set, want empty for the unverified recipient")
	}
}

func TestRecipientListVerifiedFilter(t *testing.T) {
	t.Parallel()

	client := newTestClient(t,
		mockaddy.WithRecipients(seedRecipients()),
		mockaddy.WithAliases(seedRecipientAliases()))

	// Verified only.
	m := NewRecipientListModel(client)
	m.verifiedFilter = verifyVerified
	loaded := m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("verified filter error = %v", loaded.err)
	}
	if len(loaded.recipients) != 2 {
		t.Fatalf("verified filter returned %d recipients, want 2", len(loaded.recipients))
	}

	// Unverified only.
	m.verifiedFilter = verifyUnverified
	loaded = m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("unverified filter error = %v", loaded.err)
	}
	if len(loaded.recipients) != 1 || loaded.recipients[0].id != "r3" {
		t.Fatalf("unverified filter returned %+v, want only r3", loaded.recipients)
	}
}

func TestRecipientRows(t *testing.T) {
	t.Parallel()

	verified := "2026-01-01 09:00:00"
	rows := recipientRows([]recipient{
		{id: "r1", email: "a@example.com", active: true, emailVerifiedAt: verified, aliasesCount: 2},
		{id: "r2", email: "b@example.com", aliasesCount: 0},
	})
	if len(rows) != 2 {
		t.Fatalf("recipientRows built %d rows, want 2", len(rows))
	}
	want := []table.Row{
		{"+", "+", "a@example.com", "-", "2"},
		{" ", " ", "b@example.com", "-", "0"},
	}
	for i, row := range want {
		for j, cell := range row {
			if rows[i][j] != cell {
				t.Errorf("row %d col %d = %q, want %q", i, j, rows[i][j], cell)
			}
		}
	}
}

func TestRecipientListToggleActive(t *testing.T) {
	t.Parallel()

	client := newTestClient(t,
		mockaddy.WithRecipients(seedRecipients()),
		mockaddy.WithAliases(seedRecipientAliases()))
	m := NewRecipientListModel(client)

	loaded := m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("fetch failed: %v", loaded.err)
	}

	// Toggle r1 (active) off.
	msg := m.toggleActive(loaded.recipients[0])()
	done, ok := msg.(recipientActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("toggle active failed: %v", done.err)
	}
	if done.op != "toggled active state" {
		t.Errorf("op = %q, want \"toggled active state\"", done.op)
	}
	loaded = m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("refetch failed: %v", loaded.err)
	}
	if loaded.recipients[0].active {
		t.Error("r1 is still active after the toggle")
	}

	// Toggle it back on.
	msg = m.toggleActive(loaded.recipients[0])()
	done, ok = msg.(recipientActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("toggle active failed: %v", done.err)
	}
	loaded = m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("refetch failed: %v", loaded.err)
	}
	if !loaded.recipients[0].active {
		t.Error("r1 did not become active again after the second toggle")
	}
}

func TestRecipientListToggleActiveErrorSurfaces(t *testing.T) {
	t.Parallel()

	// The recipient does not exist on the fake server, so the toggle
	// must come back as an error rather than a success.
	m := NewRecipientListModel(newTestClient(t))
	r := recipient{id: "does-not-exist", email: "gone@example.com", active: true}

	msg := m.toggleActive(r)()
	done, ok := msg.(recipientActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientActionDoneMsg", msg)
	}
	if done.err == nil {
		t.Error("toggle of a missing recipient must produce an error")
	}
}

func TestRecipientListToggleActiveNoopWithoutID(t *testing.T) {
	t.Parallel()

	m := NewRecipientListModel(newTestClient(t))
	if cmd := m.toggleActive(recipient{active: true}); cmd != nil {
		t.Error("toggleActive without an id must return nil")
	}
}

func TestRecipientListSpaceTogglesActive(t *testing.T) {
	t.Parallel()

	client := newTestClient(t,
		mockaddy.WithRecipients(seedRecipients()),
		mockaddy.WithAliases(seedRecipientAliases()))
	m := NewRecipientListModel(client)

	loaded := m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("fetch failed: %v", loaded.err)
	}
	m.recipients = loaded.recipients
	m.table.SetRows(recipientRows(loaded.recipients))

	// space on the selected row fires the toggle.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd == nil {
		t.Fatal("space must start the active toggle")
	}
	msg := cmd()
	done, ok := msg.(recipientActionDoneMsg)
	if !ok {
		t.Fatalf("command produced %T, want recipientActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("toggle active failed: %v", done.err)
	}
	if done.op != "toggled active state" {
		t.Errorf("op = %q, want \"toggled active state\"", done.op)
	}
}

func TestRecipientListSpaceWithoutSelection(t *testing.T) {
	t.Parallel()

	// space with no recipients loaded must not start a toggle.
	m := NewRecipientListModel(newTestClient(t))
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd != nil {
		t.Error("space without a selection must be a no-op")
	}
}

func TestRecipientCreateAndDelete(t *testing.T) {
	t.Parallel()

	client := newTestClient(t,
		mockaddy.WithRecipients(seedRecipients()),
		mockaddy.WithAliases(seedRecipientAliases()))
	m := NewRecipientListModel(client)

	// Create.
	msg := m.createRecipient("new@example.com")()
	done, ok := msg.(recipientActionDoneMsg)
	if !ok {
		t.Fatalf("create produced %T, want recipientActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("create failed: %v", done.err)
	}
	if done.op != "created recipient" {
		t.Errorf("op = %q, want \"created recipient\"", done.op)
	}

	// The new recipient shows up in the list, unverified.
	loaded := m.fetchRecipients()().(recipientsListLoadedMsg)
	if loaded.err != nil {
		t.Fatalf("refetch after create failed: %v", loaded.err)
	}
	var created *recipient
	for i := range loaded.recipients {
		if loaded.recipients[i].email == "new@example.com" {
			created = &loaded.recipients[i]
		}
	}
	if created == nil {
		t.Fatal("created recipient is missing from the refetched list")
	}
	if created.emailVerifiedAt != "" {
		t.Error("created recipient must start out unverified")
	}

	// Delete it.
	msg = m.deleteRecipient(*created)()
	done, ok = msg.(recipientActionDoneMsg)
	if !ok {
		t.Fatalf("delete produced %T, want recipientActionDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("delete failed: %v", done.err)
	}
	loaded = m.fetchRecipients()().(recipientsListLoadedMsg)
	for _, r := range loaded.recipients {
		if r.email == "new@example.com" {
			t.Error("deleted recipient is still in the list")
		}
	}
}
