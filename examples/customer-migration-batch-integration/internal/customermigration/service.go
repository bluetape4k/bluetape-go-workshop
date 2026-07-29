package customermigration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/gin-gonic/gin"
)

const maxRequestBodyBytes int64 = 8 << 10

var (
	errNoActiveRun = errors.New("no active run")
	runIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
)

// LeaderGate 는 leadership을 보유한 동안에만 scheduled work를 실행한다.
type LeaderGate interface {
	RunIfLeader(context.Context, func(context.Context) (RunResponse, error)) (RunResponse, error)
	LeaderHeld() bool
}

// ServiceOptions 는 운영용 service를 설정한다.
type ServiceOptions struct {
	Stores     *Stores
	LeaderGate LeaderGate
	BeforeRun  func(context.Context)
}

// Service 는 batch run 생명주기, snapshot, leader-gated operation을 소유한다.
type Service struct {
	mu                sync.Mutex
	active            bool
	cancelActive      context.CancelFunc
	stores            *Stores
	leaderGate        LeaderGate
	beforeRun         func(context.Context)
	latest            *RunResponse
	latestReport      *ReportNode
	lastRejectionCode string
}

// StartRequest 는 manual batch run을 시작한다.
type StartRequest struct {
	RunID               string `json:"run_id"`
	CrashAfterNewWrites int    `json:"crash_after_new_writes"`
}

// ScheduleRequest 는 scheduled batch tick 하나를 트리거한다.
type ScheduleRequest struct {
	RunID string `json:"run_id"`
}

// CancelRequest 는 service에 active work 취소를 요청한다.
type CancelRequest struct {
	Reason string `json:"reason"`
}

// CancelResponse 는 수락된 cancel 요청을 확인한다.
type CancelResponse struct {
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
}

// CheckpointSnapshot 은 checkpoint 존재 여부를 보고한다.
type CheckpointSnapshot struct {
	Valid bool       `json:"valid"`
	Value Checkpoint `json:"value,omitempty"`
}

// StatusResponse 는 안정적인 운영자용 status 프로젝션이다.
type StatusResponse struct {
	Active            bool               `json:"active"`
	LeaderHeld        bool               `json:"leader_held"`
	LastRejectionCode string             `json:"last_rejection_code,omitempty"`
	Checkpoint        CheckpointSnapshot `json:"checkpoint"`
	MigratedIDs       []string           `json:"migrated_ids,omitempty"`
	DeadLetters       []DeadLetter       `json:"dead_letters,omitempty"`
	Latest            *RunResponse       `json:"latest,omitempty"`
}

type errorResponse struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// NewService 는 기본적으로 in-memory store를 사용하는 service를 만든다.
func NewService(options ServiceOptions) *Service {
	stores := options.Stores.normalize()
	gate := options.LeaderGate
	if gate == nil {
		gate = NewStaticLeaderGate(true)
	}
	return &Service{
		stores:     stores,
		leaderGate: gate,
		beforeRun:  options.BeforeRun,
	}
}

// StartManual 은 운영자가 시작한 batch 하나를 실행한다.
func (s *Service) StartManual(ctx context.Context, request StartRequest) (RunResponse, error) {
	request.RunID = strings.TrimSpace(request.RunID)
	if err := validateRunID(request.RunID); err != nil {
		return s.validationResponse(request.RunID, TriggerManual, ErrorCodeInvalidRunID), err
	}
	if err := validateCrashAfter(request.CrashAfterNewWrites); err != nil {
		return s.validationResponse(request.RunID, TriggerManual, ErrorCodeInvalidCrashAfter), err
	}
	return s.run(ctx, RunOptions{
		RunID:               request.RunID,
		Trigger:             TriggerManual,
		Stores:              s.stores,
		CrashAfterNewWrites: request.CrashAfterNewWrites,
		BeforeWrite:         nil,
	})
}

// RunScheduledTick 은 leader-gated scheduled batch 하나를 실행한다.
func (s *Service) RunScheduledTick(ctx context.Context, request ScheduleRequest) (RunResponse, error) {
	request.RunID = strings.TrimSpace(request.RunID)
	if err := validateRunID(request.RunID); err != nil {
		return s.validationResponse(request.RunID, TriggerSchedule, ErrorCodeInvalidRunID), err
	}
	response, err := s.leaderGate.RunIfLeader(ctx, func(runCtx context.Context) (RunResponse, error) {
		return s.run(runCtx, RunOptions{
			RunID:   request.RunID,
			Trigger: TriggerSchedule,
			Stores:  s.stores,
		})
	})
	if err == nil {
		return response, nil
	}
	code := errorCodeFor(err)
	if errors.Is(err, ErrNotLeader) || errors.Is(err, leader.ErrNotLeader) {
		code = ErrorCodeNotLeader
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = ErrorCodeRequestCancelled
	}
	s.recordRejection(code)
	response = s.validationResponse(request.RunID, TriggerSchedule, code)
	if code == ErrorCodeRequestCancelled {
		response.Status = batch.StatusCancelled
	}
	return response, err
}

// CancelActiveRun 은 현재 active run을 취소한다.
func (s *Service) CancelActiveRun(context.Context, CancelRequest) (CancelResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active || s.cancelActive == nil {
		s.lastRejectionCode = ErrorCodeNoActiveRun
		return CancelResponse{ErrorCode: ErrorCodeNoActiveRun}, errNoActiveRun
	}
	s.cancelActive()
	return CancelResponse{Status: "cancel_requested"}, nil
}

// Status 는 defensive status snapshot을 반환한다.
func (s *Service) Status() StatusResponse {
	s.mu.Lock()
	active := s.active
	lastRejectionCode := s.lastRejectionCode
	latest := cloneRunResponsePtr(s.latest)
	s.mu.Unlock()

	checkpoint, ok := s.stores.Checkpoints.Snapshot(CheckpointKey)
	return StatusResponse{
		Active:            active,
		LeaderHeld:        s.leaderGate.LeaderHeld(),
		LastRejectionCode: lastRejectionCode,
		Checkpoint:        CheckpointSnapshot{Valid: ok, Value: checkpoint},
		MigratedIDs:       s.stores.Sink.IDs(),
		DeadLetters:       s.stores.DeadLetters.Snapshot(),
		Latest:            latest,
	}
}

// Report 는 최신 timestamp-free report 프로젝션을 반환한다.
func (s *Service) Report() (ReportNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.latestReport == nil {
		s.lastRejectionCode = ErrorCodeReportNotFound
		return ReportNode{}, fmt.Errorf("%w: report", errNoActiveRun)
	}
	return cloneReportNode(*s.latestReport), nil
}

func (s *Service) run(ctx context.Context, options RunOptions) (RunResponse, error) {
	runCtx, end, err := s.beginRun(ctx)
	if err != nil {
		response := s.validationResponse(options.RunID, options.Trigger, ErrorCodeRunInProgress)
		s.recordRejection(ErrorCodeRunInProgress)
		return response, err
	}
	defer func() {
		end()
	}()

	if s.beforeRun != nil {
		s.beforeRun(runCtx)
	}
	options.Stores = s.stores
	response, runErr := RunBatch(runCtx, options)
	s.finishRun(response)
	return response, runErr
}

func (s *Service) beginRun(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return nil, func() {}, ErrRunInProgress
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.active = true
	s.cancelActive = cancel
	return runCtx, func() {
		cancel()
	}, nil
}

func (s *Service) finishRun(response RunResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := cloneRunResponse(response)
	s.latest = &copied
	report := cloneReportNode(response.Report)
	s.latestReport = &report
	s.active = false
	s.cancelActive = nil
	if response.ErrorCode != "" {
		s.lastRejectionCode = response.ErrorCode
	}
}

func (s *Service) validationResponse(runID, trigger, code string) RunResponse {
	status := batch.StatusFailed
	if code == ErrorCodeRequestCancelled {
		status = batch.StatusCancelled
	}
	return RunResponse{
		RunID:        runID,
		Trigger:      trigger,
		Status:       status,
		MigratedIDs:  s.stores.Sink.IDs(),
		DeadLetters:  s.stores.DeadLetters.Snapshot(),
		ErrorCode:    code,
		ErrorMessage: publicMessage(code),
		FailedPhase:  "validation",
		Summary: Summary{
			ChunkSize: DefaultChunkSize,
			Failure:   true,
		},
	}
}

func (s *Service) recordRejection(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRejectionCode = code
}

func validateRunID(value string) error {
	if !runIDPattern.MatchString(value) {
		return fmt.Errorf("%w: %q", ErrInvalidRunID, value)
	}
	return nil
}

// NewRouter 는 Gin operations API를 연결한다.
func NewRouter(service *Service) (*gin.Engine, error) {
	if service == nil {
		service = NewService(ServiceOptions{})
	}
	router := gin.New()
	router.Use(gin.Recovery())
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/batch/start", func(c *gin.Context) {
		var request StartRequest
		if !bindJSON(c, &request) {
			return
		}
		response, err := service.StartManual(c.Request.Context(), request)
		writeRunResponse(c, response, err)
	})
	router.POST("/batch/schedule/tick", func(c *gin.Context) {
		var request ScheduleRequest
		if !bindJSON(c, &request) {
			return
		}
		response, err := service.RunScheduledTick(c.Request.Context(), request)
		writeRunResponse(c, response, err)
	})
	router.POST("/batch/cancel", func(c *gin.Context) {
		var request CancelRequest
		if !bindJSON(c, &request) {
			return
		}
		response, err := service.CancelActiveRun(c.Request.Context(), request)
		if err != nil {
			writeError(c, http.StatusNotFound, ErrorCodeNoActiveRun)
			return
		}
		c.JSON(http.StatusAccepted, response)
	})
	router.GET("/batch/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, service.Status())
	})
	router.GET("/batch/report", func(c *gin.Context) {
		report, err := service.Report()
		if err != nil {
			writeError(c, http.StatusNotFound, ErrorCodeReportNotFound)
			return
		}
		c.JSON(http.StatusOK, report)
	})
	return router, nil
}

func bindJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	if err := c.ShouldBindJSON(target); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(c, http.StatusRequestEntityTooLarge, ErrorCodeRequestTooLarge)
			return false
		}
		writeError(c, http.StatusBadRequest, ErrorCodeInvalidRequest)
		return false
	}
	return true
}

func writeRunResponse(c *gin.Context, response RunResponse, err error) {
	if err == nil {
		c.JSON(http.StatusOK, response)
		return
	}
	switch response.ErrorCode {
	case ErrorCodeInvalidRunID, ErrorCodeInvalidCrashAfter, ErrorCodeInvalidRequest:
		c.JSON(http.StatusBadRequest, response)
	case ErrorCodeRequestCancelled:
		c.JSON(http.StatusRequestTimeout, response)
	case ErrorCodeNotLeader, ErrorCodeRunInProgress, ErrorCodeWriterCrash, ErrorCodeTransientCustomer, ErrorCodePermanentCustomer:
		c.JSON(http.StatusConflict, response)
	default:
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			response.ErrorCode = ErrorCodeRequestCancelled
			response.ErrorMessage = publicMessage(ErrorCodeRequestCancelled)
			c.JSON(http.StatusRequestTimeout, response)
			return
		}
		c.JSON(http.StatusConflict, response)
	}
}

func writeError(c *gin.Context, status int, code string) {
	c.JSON(status, errorResponse{
		ErrorCode:    code,
		ErrorMessage: publicMessage(code),
	})
}

type staticLeaderGate struct {
	held bool
}

// NewStaticLeaderGate 는 결정적인 데모 leader gate를 만든다.
func NewStaticLeaderGate(held bool) LeaderGate {
	return staticLeaderGate{held: held}
}

func (g staticLeaderGate) RunIfLeader(ctx context.Context, run func(context.Context) (RunResponse, error)) (RunResponse, error) {
	if err := ctx.Err(); err != nil {
		return RunResponse{}, err
	}
	if !g.held {
		return RunResponse{}, ErrNotLeader
	}
	return run(ctx)
}

func (g staticLeaderGate) LeaderHeld() bool {
	return g.held
}

// ElectorLeaderGate 는 bluetape-go leader elector를 adapter로 감싼다.
type ElectorLeaderGate struct {
	elector        leader.Elector
	cleanupTimeout time.Duration
}

// NewElectorLeaderGate 는 cleanup 시간이 제한된 leader gate adapter를 만든다.
func NewElectorLeaderGate(elector leader.Elector, cleanupTimeout time.Duration) (*ElectorLeaderGate, error) {
	if elector == nil {
		return nil, fmt.Errorf("leader elector must not be nil")
	}
	if cleanupTimeout <= 0 {
		cleanupTimeout = 500 * time.Millisecond
	}
	return &ElectorLeaderGate{elector: elector, cleanupTimeout: cleanupTimeout}, nil
}

// RunIfLeader 는 이 process가 leader 소유권을 보유한 동안에만 run을 실행한다.
func (g *ElectorLeaderGate) RunIfLeader(ctx context.Context, run func(context.Context) (RunResponse, error)) (RunResponse, error) {
	if err := ctx.Err(); err != nil {
		return RunResponse{}, err
	}
	acquired := false
	if err := g.elector.Campaign(ctx); err != nil {
		switch {
		case errors.Is(err, leader.ErrAlreadyLeader):
		case errors.Is(err, leader.ErrNotLeader):
			return RunResponse{}, ErrNotLeader
		default:
			return RunResponse{}, err
		}
	} else {
		acquired = true
	}

	response, runErr := run(ctx)
	if !acquired {
		return response, runErr
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), g.cleanupTimeout)
	defer cancel()
	if err := g.elector.Resign(cleanupCtx); err != nil {
		response.Diagnostics = append(response.Diagnostics, "leader_resign_failed")
	}
	return response, runErr
}

// LeaderHeld 는 이 process가 현재 leader 소유권을 보유하는지 보고한다.
func (g *ElectorLeaderGate) LeaderHeld() bool {
	if g == nil || g.elector == nil {
		return false
	}
	return g.elector.IsLeader()
}

func cloneRunResponsePtr(response *RunResponse) *RunResponse {
	if response == nil {
		return nil
	}
	copied := cloneRunResponse(*response)
	return &copied
}

func cloneRunResponse(response RunResponse) RunResponse {
	response.ReadIDs = copyStrings(response.ReadIDs)
	response.AcceptedIDs = copyStrings(response.AcceptedIDs)
	response.NewWrittenIDs = copyStrings(response.NewWrittenIDs)
	response.MigratedIDs = copyStrings(response.MigratedIDs)
	response.DeadLetters = append([]DeadLetter(nil), response.DeadLetters...)
	response.Report = cloneReportNode(response.Report)
	return response
}

func cloneReportNode(report ReportNode) ReportNode {
	report.Children = append([]ReportNode(nil), report.Children...)
	for i := range report.Children {
		report.Children[i] = cloneReportNode(report.Children[i])
	}
	return report
}
