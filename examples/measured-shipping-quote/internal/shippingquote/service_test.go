package shippingquote

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/measure"
)

func TestServiceQuotesMetricParcel(t *testing.T) {
	service := NewService()

	quote, err := service.Quote(QuoteRequest{
		QuoteID:         "ship-metric-1001",
		DestinationZone: "kr-seoul",
		Width:           "40 cm",
		Height:          "30 cm",
		Length:          "20 cm",
		Weight:          "3.2 kg",
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	if quote.DestinationZone != "KR-SEOUL" {
		t.Fatalf("DestinationZone = %q", quote.DestinationZone)
	}
	assertMeasurement(t, quote.Dimensions.Volume, "cm^3", 24000)
	assertMeasurement(t, quote.Dimensions.FloorArea, "cm^2", 1200)
	assertMeasurement(t, quote.DimensionalWeight, "kg", 4.8)
	assertMeasurement(t, quote.BillableWeight, "kg", 4.8)
	if quote.BillableRule != "dimensional_weight" {
		t.Fatalf("BillableRule = %q", quote.BillableRule)
	}
	if quote.HandlingClass != "parcel" || !quote.Policy.Accepted {
		t.Fatalf("policy = %#v, handling = %s", quote.Policy, quote.HandlingClass)
	}
}

func TestServiceQuotesImperialDimensionsAndPounds(t *testing.T) {
	service := NewService()
	divisor := 6000.0

	quote, err := service.Quote(QuoteRequest{
		QuoteID:             "ship-imperial-1001",
		DestinationZone:     "us-west",
		Width:               "12 in",
		Height:              "10 in",
		Length:              "18 in",
		Weight:              "20 lb",
		DimensionalDivisor:  &divisor,
		DeclaredMaxSide:     "2 ft",
		PreferredLengthUnit: "in",
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMeasurement(t, quote.Dimensions.Width, "in", 12)
	assertMeasurement(t, quote.Dimensions.Length, "in", 18)
	assertMeasurement(t, quote.ActualWeight, "kg", 9.071847)
	if quote.BillableRule != "actual_weight" {
		t.Fatalf("BillableRule = %q", quote.BillableRule)
	}
}

func TestServiceMapsIncompatibleUnit(t *testing.T) {
	service := NewService()

	_, err := service.Quote(validRequest(func(request *QuoteRequest) {
		request.Width = "2 kg"
	}))
	if !errors.Is(err, ErrIncompatibleUnit) {
		t.Fatalf("Quote() error = %v, want ErrIncompatibleUnit", err)
	}
	if !errors.Is(err, measure.ErrInvalidUnit) {
		t.Fatalf("Quote() error = %v, want measure.ErrInvalidUnit", err)
	}
}

func TestServiceMapsParseFailure(t *testing.T) {
	service := NewService()

	_, err := service.Quote(validRequest(func(request *QuoteRequest) {
		request.Height = "many cm"
	}))
	if !errors.Is(err, ErrInvalidMeasure) {
		t.Fatalf("Quote() error = %v, want ErrInvalidMeasure", err)
	}
	if !errors.Is(err, measure.ErrInvalidParse) {
		t.Fatalf("Quote() error = %v, want measure.ErrInvalidParse", err)
	}
}

func TestServiceRejectsZeroDimensionalDivisor(t *testing.T) {
	_, err := dimensionalWeight(measure.Must(1, measure.VolumeCubicCentimeter()), 0)
	if !errors.Is(err, ErrInvalidDivisor) {
		t.Fatalf("dimensionalWeight() error = %v, want ErrInvalidDivisor", err)
	}
	if !errors.Is(err, measure.ErrDivideByZero) {
		t.Fatalf("dimensionalWeight() error = %v, want measure.ErrDivideByZero", err)
	}
}

func TestServiceRejectsDeclaredMaxSideBelowParsedDimensions(t *testing.T) {
	service := NewService()

	_, err := service.Quote(validRequest(func(request *QuoteRequest) {
		request.DeclaredMaxSide = "10 cm"
	}))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Quote() error = %v, want ErrInvalidRequest", err)
	}
}

func TestServiceRejectsInvalidPreferredLengthUnit(t *testing.T) {
	service := NewService()

	_, err := service.Quote(validRequest(func(request *QuoteRequest) {
		request.PreferredLengthUnit = "kg"
	}))
	if !errors.Is(err, ErrIncompatibleUnit) {
		t.Fatalf("Quote() error = %v, want ErrIncompatibleUnit", err)
	}
}

func validRequest(mutate func(*QuoteRequest)) QuoteRequest {
	request := QuoteRequest{
		QuoteID:         "ship-test-1001",
		DestinationZone: "kr-seoul",
		Width:           "40 cm",
		Height:          "30 cm",
		Length:          "20 cm",
		Weight:          "3 kg",
	}
	if mutate != nil {
		mutate(&request)
	}
	return request
}

func assertMeasurement(t *testing.T, got MeasurementValue, wantUnit string, wantAmount float64) {
	t.Helper()
	if got.Unit != wantUnit {
		t.Fatalf("Unit = %q, want %q", got.Unit, wantUnit)
	}
	if got.Amount != wantAmount {
		t.Fatalf("Amount = %v, want %v (display %q)", got.Amount, wantAmount, got.Display)
	}
	if got.Display == "" || got.Display == "<invalid>" {
		t.Fatalf("Display = %q", got.Display)
	}
}
