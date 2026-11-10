package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kovmir/addyui/internal/api"
)

// recipient is an email address that can receive forwarded mail.
type recipient struct {
	id    string
	email string
}

// recipientsLoadedMsg carries the result of the verified-recipients
// fetch. Both the create and details screens use it to fill their
// recipients pickers.
type recipientsLoadedMsg struct {
	recipients []recipient
	err        error
}

// fetchVerifiedRecipients fetches the verified recipients mail can be forwarded to.
func fetchVerifiedRecipients(client *api.ClientWithResponses) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		verified := "true" // Only verified recipients can receive mail.
		noAliasCount := "false"
		params := &api.GetAllRecipientsParams{
			FilterVerified:   &verified,
			FilterAliasCount: &noAliasCount,
		}

		resp, err := client.GetAllRecipientsWithResponse(ctx, params)
		if err != nil {
			return recipientsLoadedMsg{err: fmt.Errorf("request failed: %w", err)}
		}
		if resp.JSON200 == nil {
			return recipientsLoadedMsg{err: fmt.Errorf("unexpected response: %s", resp.Status())}
		}
		msg := recipientsLoadedMsg{}
		if data := resp.JSON200.Data; data != nil {
			for _, r := range *data {
				msg.recipients = append(msg.recipients, recipient{
					id:    str(r.Id),
					email: str(r.Email),
				})
			}
		}
		return msg
	}
}

// aliasRecipients extracts the recipients attached to an alias.
// The generated Alias type declares Recipients as an untyped []interface{};
// the API actually returns recipient objects with id and email fields.
func aliasRecipients(a *api.Alias) []recipient {
	if a.Recipients == nil {
		return nil
	}
	var out []recipient
	for _, item := range *a.Recipients {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		r := recipient{}
		if id, ok := obj["id"].(string); ok {
			r.id = id
		}
		if email, ok := obj["email"].(string); ok {
			r.email = email
		}
		if r.id != "" || r.email != "" {
			out = append(out, r)
		}
	}
	return out
}

// recipientSummary renders recipient emails for a table VALUE column.
func recipientSummary(emails []string) string {
	var names []string
	width := 0
	for _, email := range emails {
		names = append(names, email)
		width += len(email) + 2
		if width > 40 {
			// Truncate long lists.
			return strings.Join(names, ", ") + "..."
		}
	}
	if len(names) == 0 {
		return "-" // No recipients.
	}
	return strings.Join(names, ", ")
}
