// SPDX-License-Identifier: MIT

package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kovmir/addyui/internal/api"
)

// This file implements code shared between the alias screens.

// verifiedRecipientsLoadedMsg carries the result of the verified
// recipients fetch used to fill the recipients picker on the alias
// screens.
type verifiedRecipientsLoadedMsg struct {
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
			return verifiedRecipientsLoadedMsg{err: fmt.Errorf("request failed: %w", err)}
		}
		if resp.JSON200 == nil {
			return verifiedRecipientsLoadedMsg{err: fmt.Errorf("unexpected response: %s", resp.Status())}
		}
		msg := verifiedRecipientsLoadedMsg{}
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

// setAliasActive activates the alias when active is true, and
// deactivates it otherwise. It is the shared API call behind the
// toggles on the alias list and details screens.
func setAliasActive(ctx context.Context, client *api.ClientWithResponses,
	id string, active bool) error {

	if active {
		resp, err := client.ActivateASpecificAliasWithResponse(ctx,
			&api.ActivateASpecificAliasParams{},
			api.ActivateASpecificAliasJSONRequestBody{Id: id})
		return apiError(err, resp)
	}
	resp, err := client.DeactivateASpecificAliasWithResponse(ctx, id,
		&api.DeactivateASpecificAliasParams{})
	return apiError(err, resp)
}

// setAliasPin pins the alias when pin is true, and unpins it otherwise.
// It is the shared API call behind the toggles on the alias list and
// details screens.
func setAliasPin(ctx context.Context, client *api.ClientWithResponses,
	id string, pin bool) error {

	if pin {
		resp, err := client.PinASpecificAliasWithResponse(ctx,
			&api.PinASpecificAliasParams{},
			api.PinASpecificAliasJSONRequestBody{Id: id})
		return apiError(err, resp)
	}
	resp, err := client.UnpinASpecificAliasWithResponse(ctx, id,
		&api.UnpinASpecificAliasParams{})
	return apiError(err, resp)
}

// setAliasDeleted deletes the alias when deleted is true, and restores
// it otherwise. Deletion is soft: the alias can be restored later. It
// is the shared API call behind the toggles on the alias list and
// details screens.
func setAliasDeleted(ctx context.Context, client *api.ClientWithResponses,
	id string, deleted bool) error {

	if !deleted {
		resp, err := client.RestoreASpecificDeletedAliasWithResponse(ctx, id,
			&api.RestoreASpecificDeletedAliasParams{})
		return apiError(err, resp)
	}
	resp, err := client.DeleteASpecificAliasWithResponse(ctx, id,
		&api.DeleteASpecificAliasParams{})
	return apiError(err, resp)
}
