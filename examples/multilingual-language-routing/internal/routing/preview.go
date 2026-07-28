package routing

import "fmt"

// Preview 는 결정적인 라우팅 증거와 수명주기 메모를 담는다.
type Preview struct {
	Scenario              string      `json:"scenario"`
	Config                Config      `json:"config"`
	ModelLoading          string      `json:"model_loading"`
	LifecycleNotes        []string    `json:"lifecycle_notes"`
	HeuristicBoundaries   []string    `json:"heuristic_boundaries"`
	Decisions             []Decision  `json:"decisions"`
	LowConfidenceFallback PolicyCheck `json:"low_confidence_fallback"`
	TestCommands          []string    `json:"test_commands"`
}

// PolicyCheck 는 명시적인 정책 설정과 그 결정을 기록한다.
type PolicyCheck struct {
	Config   Config   `json:"config"`
	Decision Decision `json:"decision"`
}

// NewPreview 는 지연 로딩 또는 사전 로딩 감지기 모델로 고정 요청을 라우팅한다.
func NewPreview(preload bool) (Preview, error) {
	config := DefaultConfig()
	config.PreloadModels = preload
	router, err := NewRouter(config)
	if err != nil {
		return Preview{}, err
	}
	requests := []Request{
		{ID: "preview-en", Text: "Please review the delivery status for order 12345."},
		{ID: "preview-ko", Text: "주문 번호 12345의 배송 상태를 확인해 주세요."},
		{ID: "preview-ja", Text: "配送状況を確認してください。注文番号は12345です。"},
		{ID: "preview-zh", Text: "请确认订单12345的配送状态。"},
		{ID: "preview-mixed", Text: "Please check 配送状況を確認してください for order 12345."},
		{ID: "preview-short", Text: "help"},
		{ID: "preview-unknown", Text: "12345 67890"},
	}
	decisions := make([]Decision, len(requests))
	for i, request := range requests {
		decisions[i], err = router.Route(request)
		if err != nil {
			return Preview{}, fmt.Errorf("route preview %q: %w", request.ID, err)
		}
	}

	lowConfig := config
	lowConfig.MinimumConfidence = 1.0
	lowRouter, err := NewRouter(lowConfig)
	if err != nil {
		return Preview{}, fmt.Errorf("create low-confidence policy router: %w", err)
	}
	lowDecision, err := lowRouter.Route(Request{ID: "preview-low", Text: "support 문의 订单 delivery"})
	if err != nil {
		return Preview{}, fmt.Errorf("route low-confidence preview: %w", err)
	}

	loading := "lazy"
	if preload {
		loading = "preloaded"
	}
	return Preview{
		Scenario:     "route multilingual content while preserving heuristic uncertainty",
		Config:       config,
		ModelLoading: loading,
		LifecycleNotes: []string{
			"Construct one detector and reuse it across requests.",
			"Lazy loading defers model work; preloading moves it to construction without changing routes.",
			"The example gathers three public evidence views with repeated detector work and projection allocations; production policies should request only what they use.",
		},
		HeuristicBoundaries: []string{
			"Language evidence is heuristic and must not control authentication, authorization, sanctions, or compliance.",
			"Decisions retain original text for span inspection; callers own redaction, logging, telemetry, and access control.",
			"Unsupported or uncertain evidence routes to manual review rather than a processor.",
		},
		Decisions:             decisions,
		LowConfidenceFallback: PolicyCheck{Config: lowConfig, Decision: lowDecision},
		TestCommands: []string{
			"go test -count=1 ./examples/multilingual-language-routing/...",
			"go test -race -count=1 ./examples/multilingual-language-routing/...",
		},
	}, nil
}
