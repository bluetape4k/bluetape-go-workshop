// Package invites demonstrates practical string codec usage.
package invites

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/codec"
	"github.com/bluetape4k/bluetape-go/core"
)

// InvitationLink is a user-facing team invitation URL fragment.
type InvitationLink struct {
	URL   string
	Token string
}

// NewInvitationLink creates a short URL-safe invitation link.
func NewInvitationLink(baseURL string) (InvitationLink, error) {
	if err := core.RequireNotBlank("baseURL", baseURL); err != nil {
		return InvitationLink{}, err
	}
	tokenBytes := make([]byte, 12)
	if _, err := rand.Read(tokenBytes); err != nil {
		return InvitationLink{}, err
	}
	token := codec.EncodeURL62(tokenBytes)
	return InvitationLink{
		URL:   strings.TrimRight(baseURL, "/") + "/invite/" + token,
		Token: token,
	}, nil
}

// EncodeCallbackState encodes non-secret callback state for URLs.
func EncodeCallbackState(tenant, redirect string) (string, error) {
	if err := core.RequireNotBlank("tenant", tenant); err != nil {
		return "", err
	}
	payload := tenant + "|" + redirect
	return codec.EncodeBase64URL([]byte(payload)), nil
}

// DecodeCallbackState decodes callback state and validates its shape.
func DecodeCallbackState(value string) (string, string, error) {
	decoded, err := codec.DecodeBase64URL(value)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("callback state is invalid")
	}
	return parts[0], parts[1], nil
}

// ExternalReference creates a support-friendly reconciliation reference.
func ExternalReference(system string, id string) (string, error) {
	if err := core.RequireNotBlank("system", system); err != nil {
		return "", err
	}
	if err := core.RequireNotBlank("id", id); err != nil {
		return "", err
	}
	return strings.ToUpper(system) + "-" + codec.EncodeHexString(id), nil
}
