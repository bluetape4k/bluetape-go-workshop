// Package invites 는 실용적인 string codec 사용법을 보여준다.
package invites

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/codec"
	"github.com/bluetape4k/bluetape-go/core"
)

// InvitationLink 는 사용자에게 노출되는 team invitation URL fragment다.
type InvitationLink struct {
	URL   string
	Token string
}

// NewInvitationLink 는 짧고 URL-safe한 invitation link를 만든다.
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

// EncodeCallbackState 는 URL에 넣을 non-secret callback state를 encode한다.
func EncodeCallbackState(tenant, redirect string) (string, error) {
	if err := core.RequireNotBlank("tenant", tenant); err != nil {
		return "", err
	}
	payload := tenant + "|" + redirect
	return codec.EncodeBase64URL([]byte(payload)), nil
}

// DecodeCallbackState 는 callback state를 decode하고 shape를 검증한다.
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

// ExternalReference 는 support-friendly reconciliation reference를 만든다.
func ExternalReference(system string, id string) (string, error) {
	if err := core.RequireNotBlank("system", system); err != nil {
		return "", err
	}
	if err := core.RequireNotBlank("id", id); err != nil {
		return "", err
	}
	return strings.ToUpper(system) + "-" + codec.EncodeHexString(id), nil
}
