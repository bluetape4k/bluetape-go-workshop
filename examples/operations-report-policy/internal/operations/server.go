// Package operations 는 workreport 실패 정책을 위한 작은 Gin API를 노출한다.
package operations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/gin-gonic/gin"
)

const (
	policyStopOnFailureLabel     = "stop_on_failure"
	policyContinueOnFailureLabel = "continue_on_failure"
)

var (
	// ErrProductValidation 은 상품 검증 단계가 실패했음을 나타낸다.
	ErrProductValidation = errors.New("product validation failed")
	// ErrPartnerNotification 은 파트너 알림 시도가 실패했음을 나타낸다.
	ErrPartnerNotification = errors.New("partner notification failed")

	errInvalidRunID        = errors.New("run_id is required")
	errInvalidPolicy       = errors.New("policy must be stop_on_failure or continue_on_failure")
	errMissingProductsFlag = errors.New("products_valid is required")
)

// Options 는 운영 리포트 정책 API 서버를 설정한다.
type Options struct{}

// Server 는 운영 리포트 집계를 HTTP로 노출한다.
type Server struct {
	router *gin.Engine
}

// RunRequest 는 하나의 운영 리포트 시나리오를 설명한다.
type RunRequest struct {
	RunID                    string `json:"run_id"`
	Policy                   string `json:"policy"`
	ProductsValid            *bool  `json:"products_valid"`
	RetryPartnerNotification bool   `json:"retry_partner_notification"`
	SkipSearchIndex          bool   `json:"skip_search_index"`
}

type runResponse struct {
	RunID   string     `json:"run_id"`
	Policy  string     `json:"policy"`
	Status  string     `json:"status"`
	Summary summaryDTO `json:"summary"`
	Report  reportNode `json:"report"`
}

type summaryDTO struct {
	Completed int  `json:"completed"`
	Failed    int  `json:"failed"`
	Partial   int  `json:"partial"`
	Aborted   int  `json:"aborted"`
	Cancelled int  `json:"cancelled"`
	Total     int  `json:"total"`
	Terminal  bool `json:"terminal"`
	Success   bool `json:"success"`
	Failure   bool `json:"failure"`
}

type reportNode struct {
	Name      string            `json:"name"`
	Status    workreport.Status `json:"status"`
	Error     string            `json:"error,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Success   bool              `json:"success"`
	Failure   bool              `json:"failure"`
	Partial   bool              `json:"partial"`
	Cancelled bool              `json:"cancelled"`
	Children  []reportNode      `json:"children,omitempty"`
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

type operationsRun struct {
	request RunRequest
	policy  workreport.FailurePolicy
}

// NewServer 는 운영 리포트 정책 API를 생성한다.
func NewServer(Options) (*Server, error) {
	router := gin.New()
	router.Use(gin.Recovery())

	server := &Server{router: router}
	router.GET("/healthz", server.health)
	router.POST("/operations/report", server.run)

	return server, nil
}

// ServeHTTP 는 요청을 Gin 라우터로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) run(c *gin.Context) {
	var request RunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	request.RunID = strings.TrimSpace(request.RunID)
	request.Policy = strings.TrimSpace(request.Policy)

	policy, err := validateRequest(request)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	report, err := operationsRun{request: request, policy: policy}.run(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	c.JSON(statusForReport(report), runResponse{
		RunID:   request.RunID,
		Policy:  request.Policy,
		Status:  string(report.Status),
		Summary: summarizeReport(report),
		Report:  projectReport(report),
	})
}

func validateRequest(request RunRequest) (workreport.FailurePolicy, error) {
	if request.RunID == "" {
		return 0, errInvalidRunID
	}
	if request.ProductsValid == nil {
		return 0, errMissingProductsFlag
	}
	policy, err := parsePolicy(request.Policy)
	if err != nil {
		return 0, err
	}
	return policy, nil
}

func parsePolicy(value string) (workreport.FailurePolicy, error) {
	switch value {
	case policyStopOnFailureLabel:
		return workreport.StopOnFailure, nil
	case policyContinueOnFailureLabel:
		return workreport.ContinueOnFailure, nil
	default:
		return 0, fmt.Errorf("%w: got %q", errInvalidPolicy, value)
	}
}

func (r operationsRun) run(ctx context.Context) (workreport.Report, error) {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("operations-run", err), nil
	}

	steps := make([]workreport.Report, 0, 5)
	for _, step := range []func(context.Context) workreport.Report{
		r.loadCatalogSnapshot,
		r.validateProducts,
		r.notifyPartner,
		r.refreshSearchIndex,
		r.writeRunSummary,
	} {
		report := step(ctx)
		steps = append(steps, report)
		if r.policy == workreport.StopOnFailure && report.Status != workreport.StatusCompleted {
			break
		}
	}

	return workreport.Aggregate("operations-run", r.policy, steps...)
}

func (r operationsRun) loadCatalogSnapshot(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("load-catalog-snapshot", err)
	}
	return workreport.Completed("load-catalog-snapshot")
}

func (r operationsRun) validateProducts(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("validate-products", err)
	}
	if !*r.request.ProductsValid {
		return workreport.Failed("validate-products", ErrProductValidation)
	}
	return workreport.Completed("validate-products")
}

func (r operationsRun) notifyPartner(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("notify-partner", err)
	}
	if !r.request.RetryPartnerNotification {
		return workreport.Completed("notify-partner")
	}

	report, err := workreport.Aggregate(
		"notify-partner",
		workreport.ContinueOnFailure,
		workreport.Failed("notify-partner-attempt-1", ErrPartnerNotification),
		workreport.Completed("notify-partner-attempt-2"),
	)
	if err != nil {
		return workreport.Failed("notify-partner", err)
	}
	return report
}

func (r operationsRun) refreshSearchIndex(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("refresh-search-index", err)
	}
	if r.request.SkipSearchIndex {
		return workreport.Aborted("refresh-search-index", "skipped by request")
	}
	return workreport.Completed("refresh-search-index")
}

func (r operationsRun) writeRunSummary(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("write-run-summary", err)
	}
	return workreport.Completed("write-run-summary")
}

func statusForReport(report workreport.Report) int {
	switch {
	case report.IsCancelled():
		return http.StatusRequestTimeout
	case report.IsSuccess():
		return http.StatusOK
	case report.IsPartial():
		return http.StatusMultiStatus
	case report.IsFailure():
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func projectReport(report workreport.Report) reportNode {
	node := reportNode{
		Name:      report.Name,
		Status:    report.Status,
		Reason:    report.Reason,
		Success:   report.IsSuccess(),
		Failure:   report.IsFailure(),
		Partial:   report.IsPartial(),
		Cancelled: report.IsCancelled(),
	}
	if report.Err != nil {
		node.Error = report.Err.Error()
	}
	if len(report.Children) > 0 {
		node.Children = make([]reportNode, 0, len(report.Children))
		for _, child := range report.Children {
			node.Children = append(node.Children, projectReport(child))
		}
	}
	return node
}

func summarizeReport(report workreport.Report) summaryDTO {
	summary := summaryDTO{
		Terminal: report.IsTerminal(),
		Success:  report.IsSuccess(),
		Failure:  report.IsFailure(),
	}
	walkReports(report, func(current workreport.Report) {
		summary.Total++
		switch current.Status {
		case workreport.StatusCompleted:
			summary.Completed++
		case workreport.StatusFailed:
			summary.Failed++
		case workreport.StatusPartial:
			summary.Partial++
		case workreport.StatusAborted:
			summary.Aborted++
		case workreport.StatusCancelled:
			summary.Cancelled++
		}
	})
	return summary
}

func walkReports(report workreport.Report, visit func(workreport.Report)) {
	visit(report)
	for _, child := range report.Children {
		walkReports(child, visit)
	}
}

func writeError(c *gin.Context, status int, code string, err error) {
	c.JSON(status, errorResponse{Code: code, Error: err.Error()})
}
