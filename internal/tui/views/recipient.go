// SPDX-License-Identifier: MIT

package views

import (
	"context"

	"github.com/kovmir/addyui/internal/api"
)

// This file implements code shared between the recipient screens.

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

// setRecipientActive activates the recipient when active is true, and
// deactivates it otherwise. It is the shared API call behind the
// toggle on the recipient list and details screens.
func setRecipientActive(ctx context.Context, client *api.ClientWithResponses,
	id string, active bool) error {

	if active {
		resp, err := client.ActivateASpecificRecipientWithResponse(ctx,
			&api.ActivateASpecificRecipientParams{},
			api.ActivateASpecificRecipientJSONRequestBody{Id: id})
		return apiError(err, resp)
	}
	resp, err := client.DeactivateASpecificRecipientWithResponse(ctx, id,
		&api.DeactivateASpecificRecipientParams{})
	return apiError(err, resp)
}
