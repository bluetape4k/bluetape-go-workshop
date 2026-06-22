package dedupe

import (
	"errors"
	"sync"
	"testing"
)

func TestServiceAdmitsDefinitelyNewEvent(t *testing.T) {
	service := newTestService(t)

	decision, err := service.Admit(EventRequest{
		EventID: " evt-1001 ",
		Source:  "checkout",
	})
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	if decision.EventID != "evt-1001" {
		t.Fatalf("event_id = %q", decision.EventID)
	}
	if decision.Decision != DecisionAdmit {
		t.Fatalf("decision = %q, want %q", decision.Decision, DecisionAdmit)
	}
	if decision.Reason != ReasonDefinitelyNew {
		t.Fatalf("reason = %q, want %q", decision.Reason, ReasonDefinitelyNew)
	}
	if !decision.Accepted {
		t.Fatalf("accepted = false, want true")
	}
	if decision.Stats.ApproximateElementCount == 0 {
		t.Fatalf("approximate_element_count = 0, want > 0")
	}
	if decision.Stats.ExpectedFalsePositiveProbability < 0 {
		t.Fatalf("expected_false_positive_probability = %v, want >= 0", decision.Stats.ExpectedFalsePositiveProbability)
	}
}

func TestServiceMarksRepeatedEventAsProbablySeen(t *testing.T) {
	service := newTestService(t)

	first, err := service.Admit(EventRequest{EventID: "evt-1002", Source: "checkout"})
	if err != nil {
		t.Fatalf("first Admit() error = %v", err)
	}
	second, err := service.Admit(EventRequest{EventID: "evt-1002", Source: "checkout"})
	if err != nil {
		t.Fatalf("second Admit() error = %v", err)
	}

	if first.Decision != DecisionAdmit || !first.Accepted {
		t.Fatalf("first decision = %+v, want admitted", first)
	}
	if second.Decision != DecisionProbablySeen {
		t.Fatalf("second decision = %q, want %q", second.Decision, DecisionProbablySeen)
	}
	if second.Reason != ReasonMightBeDuplicate {
		t.Fatalf("second reason = %q, want %q", second.Reason, ReasonMightBeDuplicate)
	}
	if second.Accepted {
		t.Fatalf("second accepted = true, want false")
	}
}

func TestServiceRejectsInvalidEventID(t *testing.T) {
	service := newTestService(t)

	_, err := service.Admit(EventRequest{EventID: "  ", Source: "checkout"})

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Admit() error = %v, want ErrInvalidRequest", err)
	}
}

func TestServiceStatsReflectAdmissions(t *testing.T) {
	service := newTestService(t)

	for _, eventID := range []string{"evt-1003", "evt-1004", "evt-1005"} {
		if _, err := service.Admit(EventRequest{EventID: eventID, Source: "checkout"}); err != nil {
			t.Fatalf("Admit(%s) error = %v", eventID, err)
		}
	}

	stats := service.Stats()
	if stats.ExpectedInsertions != 100 {
		t.Fatalf("expected_insertions = %d, want 100", stats.ExpectedInsertions)
	}
	if stats.TargetFalsePositiveProbability != 0.01 {
		t.Fatalf("target_false_positive_probability = %v, want 0.01", stats.TargetFalsePositiveProbability)
	}
	if stats.ApproximateElementCount == 0 {
		t.Fatalf("approximate_element_count = 0, want > 0")
	}
	if stats.ExpectedFalsePositiveProbability < 0 || stats.ExpectedFalsePositiveProbability > 0.01 {
		t.Fatalf("expected_false_positive_probability = %v, want between 0 and 0.01", stats.ExpectedFalsePositiveProbability)
	}
}

func TestServiceAdmitsConcurrentDuplicateOnlyOnce(t *testing.T) {
	service := newTestService(t)
	const goroutines = 32

	var wg sync.WaitGroup
	results := make(chan AdmitResponse, goroutines)
	errorsCh := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := service.Admit(EventRequest{EventID: "evt-concurrent-1001", Source: "checkout"})
			if err != nil {
				errorsCh <- err
				return
			}
			results <- response
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		t.Fatalf("Admit() error = %v", err)
	}
	admitted := 0
	probablySeen := 0
	for response := range results {
		switch response.Decision {
		case DecisionAdmit:
			admitted++
		case DecisionProbablySeen:
			probablySeen++
		default:
			t.Fatalf("unexpected decision = %+v", response)
		}
	}
	if admitted != 1 {
		t.Fatalf("admitted = %d, want 1", admitted)
	}
	if probablySeen != goroutines-1 {
		t.Fatalf("probablySeen = %d, want %d", probablySeen, goroutines-1)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService(Config{
		ExpectedInsertions:       100,
		FalsePositiveProbability: 0.01,
		ScenarioName:             "test-webhook-dedupe",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
