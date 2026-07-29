// Package shippingquote 는 계측 기반 배송 견적 워크숍 예제를 구현한다.
package shippingquote

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/bluetape4k/bluetape-go/measure"
	"github.com/gin-gonic/gin"
)

const (
	maxJSONBodySize             = 8 << 10
	defaultDimensionalDivisor   = 5000
	defaultOversizeSideCM       = 100
	defaultFreightSideCM        = 150
	defaultDimensionalDivisorCM = "cm3_per_kg"
)

var (
	// ErrInvalidRequest 는 요청 JSON 또는 필수 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("shippingquote: invalid request")
	// ErrInvalidMeasure 는 측정값 파싱 또는 값 검증 실패를 나타낸다.
	ErrInvalidMeasure = errors.New("shippingquote: invalid measure")
	// ErrIncompatibleUnit 는 문법은 올바르지만 차원이 맞지 않는 단위를 나타낸다.
	ErrIncompatibleUnit = errors.New("shippingquote: incompatible unit")
	// ErrInvalidDivisor 는 부피 무게 제수가 유효하지 않음을 나타낸다.
	ErrInvalidDivisor = errors.New("shippingquote: invalid dimensional divisor")
)

var (
	massPound        = measure.MustUnit[measure.Mass]("pound", "lb", 453.59237)
	shippingMassUnit = measure.MustRegistry(measure.MassGram(), measure.MassKilogram(), measure.MassTon(), massPound)
)

// Service 는 결정적인 배송 측정 견적을 생성한다.
type Service struct{}

// QuoteRequest 는 하나의 소포 측정 견적을 요청하는 공개 입력이다.
type QuoteRequest struct {
	QuoteID             string   `json:"quote_id"`
	DestinationZone     string   `json:"destination_zone"`
	Width               string   `json:"width"`
	Height              string   `json:"height"`
	Length              string   `json:"length"`
	Weight              string   `json:"weight"`
	DimensionalDivisor  *float64 `json:"dimensional_divisor,omitempty"`
	DeclaredMaxSide     string   `json:"declared_max_side,omitempty"`
	PreferredLengthUnit string   `json:"preferred_length_unit,omitempty"`
}

// QuoteResponse 는 안정적으로 공개되는 견적 응답 표현이다.
type QuoteResponse struct {
	QuoteID            string             `json:"quote_id"`
	DestinationZone    string             `json:"destination_zone"`
	Dimensions         DimensionSummary   `json:"dimensions"`
	ActualWeight       MeasurementValue   `json:"actual_weight"`
	DimensionalWeight  MeasurementValue   `json:"dimensional_weight"`
	BillableWeight     MeasurementValue   `json:"billable_weight"`
	BillableRule       string             `json:"billable_rule"`
	HandlingClass      string             `json:"handling_class"`
	DimensionalDivisor DimensionalDivisor `json:"dimensional_divisor"`
	Policy             PolicyDecision     `json:"policy"`
}

// DimensionSummary 는 파싱된 소포 치수와 파생 치수를 묶는다.
type DimensionSummary struct {
	Width       MeasurementValue `json:"width"`
	Height      MeasurementValue `json:"height"`
	Length      MeasurementValue `json:"length"`
	LongestSide MeasurementValue `json:"longest_side"`
	FloorArea   MeasurementValue `json:"floor_area"`
	Volume      MeasurementValue `json:"volume"`
}

// MeasurementValue 는 타입이 지정된 측정값을 위한 예제의 안정적인 JSON 형태다.
type MeasurementValue struct {
	Amount  float64 `json:"amount"`
	Unit    string  `json:"unit"`
	Display string  `json:"display"`
}

// DimensionalDivisor 는 부피가 부피 무게로 변환되는 기준을 문서화한다.
type DimensionalDivisor struct {
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

// PolicyDecision 은 측정값에 대한 비즈니스 해석을 기록한다.
type PolicyDecision struct {
	Accepted bool   `json:"accepted"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// ErrorResponse 는 안정적으로 공개되는 오류 응답 형태다.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService 는 측정 견적 서비스를 생성한다.
func NewService() *Service {
	return &Service{}
}

// Quote 는 호출자가 보낸 단위를 파싱하고 파생 배송 측정값을 반환한다.
func (s *Service) Quote(request QuoteRequest) (QuoteResponse, error) {
	if s == nil {
		return QuoteResponse{}, fmt.Errorf("%w: service is nil", ErrInvalidRequest)
	}
	quoteID := strings.TrimSpace(request.QuoteID)
	if quoteID == "" {
		return QuoteResponse{}, fmt.Errorf("%w: quote_id is required", ErrInvalidRequest)
	}
	zone := strings.ToUpper(strings.TrimSpace(request.DestinationZone))
	if zone == "" {
		return QuoteResponse{}, fmt.Errorf("%w: destination_zone is required", ErrInvalidRequest)
	}

	width, err := parseLength("width", request.Width)
	if err != nil {
		return QuoteResponse{}, err
	}
	height, err := parseLength("height", request.Height)
	if err != nil {
		return QuoteResponse{}, err
	}
	length, err := parseLength("length", request.Length)
	if err != nil {
		return QuoteResponse{}, err
	}
	weight, err := parseMass("weight", request.Weight)
	if err != nil {
		return QuoteResponse{}, err
	}
	if err := requirePositiveLengths(width, height, length); err != nil {
		return QuoteResponse{}, err
	}
	if err := requirePositiveMass(weight); err != nil {
		return QuoteResponse{}, err
	}

	lengthUnit, err := displayLengthUnit(request.PreferredLengthUnit)
	if err != nil {
		return QuoteResponse{}, err
	}
	maxSide, err := longest(width, height, length)
	if err != nil {
		return QuoteResponse{}, err
	}
	if request.DeclaredMaxSide != "" {
		declared, err := parseLength("declared_max_side", request.DeclaredMaxSide)
		if err != nil {
			return QuoteResponse{}, err
		}
		if err := requirePositiveLengths(declared); err != nil {
			return QuoteResponse{}, err
		}
		cmp, err := maxSide.Compare(declared)
		if err != nil {
			return QuoteResponse{}, mapMeasureError("declared_max_side", err)
		}
		if cmp > 0 {
			return QuoteResponse{}, fmt.Errorf("%w: declared_max_side is smaller than parsed dimensions", ErrInvalidRequest)
		}
	}

	area, err := measure.AreaFromLength(width, height)
	if err != nil {
		return QuoteResponse{}, mapMeasureError("area", err)
	}
	volume, err := measure.VolumeFromAreaLength(area, length)
	if err != nil {
		return QuoteResponse{}, mapMeasureError("volume", err)
	}
	divisor, err := dimensionalDivisor(request.DimensionalDivisor)
	if err != nil {
		return QuoteResponse{}, err
	}
	dimensionalWeight, err := dimensionalWeight(volume, divisor)
	if err != nil {
		return QuoteResponse{}, err
	}
	billable, rule, err := billableWeight(weight, dimensionalWeight)
	if err != nil {
		return QuoteResponse{}, err
	}
	handling, policy, err := handlingClass(maxSide)
	if err != nil {
		return QuoteResponse{}, err
	}

	return QuoteResponse{
		QuoteID:         quoteID,
		DestinationZone: zone,
		Dimensions: DimensionSummary{
			Width:       lengthValue(width, lengthUnit),
			Height:      lengthValue(height, lengthUnit),
			Length:      lengthValue(length, lengthUnit),
			LongestSide: lengthValue(maxSide, lengthUnit),
			FloorArea:   areaValue(area),
			Volume:      volumeValue(volume),
		},
		ActualWeight:       massValue(weight),
		DimensionalWeight:  massValue(dimensionalWeight),
		BillableWeight:     massValue(billable),
		BillableRule:       rule,
		HandlingClass:      handling,
		DimensionalDivisor: DimensionalDivisor{Amount: divisor, Unit: defaultDimensionalDivisorCM},
		Policy:             policy,
	}, nil
}

// NewRouter 는 예제용 HTTP 라우터를 생성한다.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/quotes", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request QuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid quote request")
			return
		}
		quote, err := service.Quote(request)
		if err != nil {
			writeQuoteError(c, err)
			return
		}
		c.JSON(http.StatusOK, quote)
	})

	return router
}

func parseLength(field, value string) (measure.Measure[measure.Length], error) {
	parsed, err := measure.ParseLength(strings.TrimSpace(value))
	if err != nil {
		return measure.Measure[measure.Length]{}, mapMeasureError(field, err)
	}
	return parsed, nil
}

func parseMass(field, value string) (measure.Measure[measure.Mass], error) {
	parsed, err := measure.Parse(strings.TrimSpace(value), shippingMassUnit)
	if err != nil {
		return measure.Measure[measure.Mass]{}, mapMeasureError(field, err)
	}
	return parsed, nil
}

func requirePositiveLengths(values ...measure.Measure[measure.Length]) error {
	for _, value := range values {
		cm, err := value.In(measure.LengthCentimeter())
		if err != nil {
			return mapMeasureError("length", err)
		}
		if cm <= 0 {
			return fmt.Errorf("%w: length must be positive", ErrInvalidMeasure)
		}
	}
	return nil
}

func requirePositiveMass(value measure.Measure[measure.Mass]) error {
	kg, err := value.In(measure.MassKilogram())
	if err != nil {
		return mapMeasureError("weight", err)
	}
	if kg <= 0 {
		return fmt.Errorf("%w: weight must be positive", ErrInvalidMeasure)
	}
	return nil
}

func displayLengthUnit(value string) (measure.Unit[measure.Length], error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "cm":
		return measure.LengthCentimeter(), nil
	case "m":
		return measure.LengthMeter(), nil
	case "in":
		return measure.LengthInch(), nil
	case "ft":
		return measure.LengthFoot(), nil
	default:
		return measure.Unit[measure.Length]{}, fmt.Errorf("%w: unsupported preferred_length_unit", ErrIncompatibleUnit)
	}
}

func longest(values ...measure.Measure[measure.Length]) (measure.Measure[measure.Length], error) {
	if len(values) == 0 {
		return measure.Measure[measure.Length]{}, fmt.Errorf("%w: no dimensions", ErrInvalidRequest)
	}
	longestValue := values[0]
	for _, value := range values[1:] {
		cmp, err := value.Compare(longestValue)
		if err != nil {
			return measure.Measure[measure.Length]{}, mapMeasureError("longest_side", err)
		}
		if cmp > 0 {
			longestValue = value
		}
	}
	return longestValue, nil
}

func dimensionalDivisor(value *float64) (float64, error) {
	if value == nil {
		return defaultDimensionalDivisor, nil
	}
	divisor := *value
	if math.IsNaN(divisor) || math.IsInf(divisor, 0) {
		return 0, fmt.Errorf("%w: divisor must be finite", ErrInvalidDivisor)
	}
	if divisor < 0 {
		return 0, fmt.Errorf("%w: divisor must be positive", ErrInvalidDivisor)
	}
	if divisor == 0 {
		return 0, fmt.Errorf("%w: %w", ErrInvalidDivisor, measure.ErrDivideByZero)
	}
	return divisor, nil
}

func dimensionalWeight(volume measure.Measure[measure.Volume], divisor float64) (measure.Measure[measure.Mass], error) {
	cubicCM, err := volume.As(measure.VolumeCubicCentimeter())
	if err != nil {
		return measure.Measure[measure.Mass]{}, mapMeasureError("volume", err)
	}
	kg, err := measure.New(cubicCM.Amount(), measure.MassKilogram())
	if err != nil {
		return measure.Measure[measure.Mass]{}, mapMeasureError("dimensional_weight", err)
	}
	kg, err = kg.DivScalar(divisor)
	if err != nil {
		return measure.Measure[measure.Mass]{}, fmt.Errorf("%w: %w", ErrInvalidDivisor, err)
	}
	return kg, nil
}

func billableWeight(actual, dimensional measure.Measure[measure.Mass]) (measure.Measure[measure.Mass], string, error) {
	cmp, err := dimensional.Compare(actual)
	if err != nil {
		return measure.Measure[measure.Mass]{}, "", mapMeasureError("billable_weight", err)
	}
	if cmp > 0 {
		return dimensional, "dimensional_weight", nil
	}
	return actual, "actual_weight", nil
}

func handlingClass(longestSide measure.Measure[measure.Length]) (string, PolicyDecision, error) {
	cm, err := longestSide.In(measure.LengthCentimeter())
	if err != nil {
		return "", PolicyDecision{}, mapMeasureError("handling_class", err)
	}
	switch {
	case cm > defaultFreightSideCM:
		return "freight", PolicyDecision{
			Accepted: false,
			Code:     "freight_required",
			Message:  "longest side exceeds parcel service limit",
		}, nil
	case cm > defaultOversizeSideCM:
		return "oversize", PolicyDecision{
			Accepted: true,
			Code:     "oversize_surcharge",
			Message:  "longest side requires an oversize parcel lane",
		}, nil
	default:
		return "parcel", PolicyDecision{
			Accepted: true,
			Code:     "standard_parcel",
			Message:  "parcel can use the standard measured quote lane",
		}, nil
	}
}

func lengthValue(value measure.Measure[measure.Length], unit measure.Unit[measure.Length]) MeasurementValue {
	return measurementValue(value, unit)
}

func areaValue(value measure.Measure[measure.Area]) MeasurementValue {
	return measurementValue(value, measure.AreaSquareCentimeter())
}

func volumeValue(value measure.Measure[measure.Volume]) MeasurementValue {
	return measurementValue(value, measure.VolumeCubicCentimeter())
}

func massValue(value measure.Measure[measure.Mass]) MeasurementValue {
	return measurementValue(value, measure.MassKilogram())
}

func measurementValue[D any](value measure.Measure[D], unit measure.Unit[D]) MeasurementValue {
	amount, err := value.In(unit)
	if err != nil {
		return MeasurementValue{Unit: unit.Suffix(), Display: "<invalid>"}
	}
	display, err := value.Format(unit)
	if err != nil {
		display = "<invalid>"
	}
	return MeasurementValue{
		Amount:  math.Round(amount*1_000_000) / 1_000_000,
		Unit:    unit.Suffix(),
		Display: display,
	}
}

func mapMeasureError(field string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, measure.ErrInvalidUnit), errors.Is(err, measure.ErrIncompatibleUnit):
		return fmt.Errorf("%w: %s: %w", ErrIncompatibleUnit, field, err)
	case errors.Is(err, measure.ErrDivideByZero):
		return fmt.Errorf("%w: %s: %w", ErrInvalidDivisor, field, err)
	case errors.Is(err, measure.ErrInvalidMeasure), errors.Is(err, measure.ErrInvalidParse):
		return fmt.Errorf("%w: %s: %w", ErrInvalidMeasure, field, err)
	default:
		return err
	}
}

func writeQuoteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid quote request")
	case errors.Is(err, ErrIncompatibleUnit):
		writeError(c, http.StatusBadRequest, "incompatible_unit", "measurement unit is incompatible with the expected field")
	case errors.Is(err, ErrInvalidDivisor):
		writeError(c, http.StatusBadRequest, "invalid_dimensional_divisor", "invalid dimensional weight divisor")
	case errors.Is(err, ErrInvalidMeasure):
		writeError(c, http.StatusBadRequest, "invalid_measure", "invalid measurement input")
	default:
		writeError(c, http.StatusInternalServerError, "quote_failed", "shipping quote failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
