package moderation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch"
)

func TestEvaluateMasksOverlappingAndKoreanContent(t *testing.T) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Evaluate(context.Background(), ReviewRequest{
		ContentID: "comment-1001",
		Text:      "bad wolf offer와 욕설 포함",
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.Decision != DecisionMasked {
		t.Fatalf("Decision = %q, want %q", result.Decision, DecisionMasked)
	}
	if result.MaskedText != "******** offer와 ** 포함" {
		t.Fatalf("MaskedText = %q", result.MaskedText)
	}
	gotIDs := findingIDs(result.Findings)
	wantIDs := []string{"abuse-bad-wolf", "ko-abuse"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("finding ids = %#v, want %#v", gotIDs, wantIDs)
	}
	if result.Findings[0].Severity != textsearch.SeverityHigh {
		t.Fatalf("bad wolf severity = %v, want high", result.Findings[0].Severity)
	}
}

func TestEvaluateHonorsAllowlistBeforeMasking(t *testing.T) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Evaluate(context.Background(), ReviewRequest{
		ContentID: "comment-1002",
		Text:      "bad wolf book club discusses scam risk",
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.MaskedText != "bad wolf book club discusses **** risk" {
		t.Fatalf("MaskedText = %q", result.MaskedText)
	}
	if got := findingIDs(result.Findings); !reflect.DeepEqual(got, []string{"fraud-scam"}) {
		t.Fatalf("finding ids = %#v", got)
	}
	if len(result.AllowlistHits) != 1 || result.AllowlistHits[0].ID != "title-bad-wolf-book-club" {
		t.Fatalf("AllowlistHits = %#v", result.AllowlistHits)
	}
}

func TestEvaluateKeepsReplacementBoundaries(t *testing.T) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Evaluate(context.Background(), ReviewRequest{
		ContentID: "comment-1003",
		Text:      "badge bad badwolf bad.",
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.MaskedText != "badge *** badwolf ***." {
		t.Fatalf("MaskedText = %q", result.MaskedText)
	}
	if got := findingIDs(result.Findings); !reflect.DeepEqual(got, []string{"abuse-bad", "abuse-bad"}) {
		t.Fatalf("finding ids = %#v", got)
	}
}

func TestEvaluateReturnsCallerCancellation(t *testing.T) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = service.Evaluate(ctx, ReviewRequest{
		ContentID: "comment-1004",
		Text:      "scam",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Evaluate() error = %v, want context.Canceled", err)
	}
}

func TestPreviewDocumentsScenario(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}

	if preview.Scenario != "review marketplace comments with deterministic blockword masking" {
		t.Fatalf("Scenario = %q", preview.Scenario)
	}
	if len(preview.Samples) != 3 {
		t.Fatalf("sample count = %d, want 3", len(preview.Samples))
	}
	if preview.Samples[0].MaskedText != "******** offer와 ** 포함" {
		t.Fatalf("first sample masked text = %q", preview.Samples[0].MaskedText)
	}
}

func findingIDs(findings []Finding) []string {
	ids := make([]string, len(findings))
	for i, finding := range findings {
		ids[i] = finding.ID
	}
	return ids
}
