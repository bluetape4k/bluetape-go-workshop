package invites_test

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/invitation-codecs/internal/invites"
)

func TestInvitationLinkUsesURLSafeToken(t *testing.T) {
	link, err := invites.NewInvitationLink("https://app.example.test/")
	if err != nil {
		t.Fatalf("new invitation link: %v", err)
	}
	if !strings.HasPrefix(link.URL, "https://app.example.test/invite/") {
		t.Fatalf("url = %s", link.URL)
	}
	if strings.ContainsAny(link.Token, "+/=") {
		t.Fatalf("token should be URL-safe, got %s", link.Token)
	}
}

func TestCallbackStateRoundTrip(t *testing.T) {
	encoded, err := invites.EncodeCallbackState("tenant-a", "/settings/team")
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
	tenant, redirect, err := invites.DecodeCallbackState(encoded)
	if err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if tenant != "tenant-a" || redirect != "/settings/team" {
		t.Fatalf("tenant=%q redirect=%q", tenant, redirect)
	}
}

func TestMalformedCallbackStateFails(t *testing.T) {
	if _, _, err := invites.DecodeCallbackState("not base64"); err == nil {
		t.Fatal("expected malformed state to fail")
	}
}

func TestExternalReferenceIsReadableButNotSecret(t *testing.T) {
	reference, err := invites.ExternalReference("erp", "order-42")
	if err != nil {
		t.Fatalf("external reference: %v", err)
	}
	if !strings.HasPrefix(reference, "ERP-") {
		t.Fatalf("reference = %s", reference)
	}
	if strings.Contains(reference, "order-42") {
		t.Fatalf("hex reference should not expose raw id directly, got %s", reference)
	}
}
