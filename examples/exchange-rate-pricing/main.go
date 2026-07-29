// Package main 은 provider-backed exchange-rate pricing 예제 서비스를 실행한다.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/exchange-rate-pricing/internal/exchangepricing"
	"github.com/bluetape4k/bluetape-go/money"
	"github.com/gin-gonic/gin"
)

const defaultHTTPAddr = "127.0.0.1:8101"

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)

	provider := newDemoRateProvider()
	server := newHTTPServer(addr, exchangepricing.NewRouter(exchangepricing.NewService(provider)))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("exchange-rate pricing example listening on http://%s", addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func resolveHTTPAddr(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultHTTPAddr
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("port is required")
	}
	if host == "localhost" {
		return value, nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", err
	}
	if !addr.IsLoopback() {
		return "", fmt.Errorf("HTTP_ADDR must bind to loopback, got %q", value)
	}
	return value, nil
}

func newDemoRateProvider() money.ExchangeRateProvider {
	return staticRateProvider{
		rates: map[string]string{
			rateKey(money.USD, money.KRW): "1300",
			rateKey(money.USD, money.EUR): "0.92",
			rateKey(money.EUR, money.KRW): "1410",
			rateKey(money.EUR, money.USD): "1.08",
		},
		now: func() time.Time {
			return time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
		},
	}
}

type staticRateProvider struct {
	rates map[string]string
	now   func() time.Time
}

func (p staticRateProvider) Rate(ctx context.Context, base money.Currency, target money.Currency) (money.ExchangeRateQuote, error) {
	if err := ctx.Err(); err != nil {
		return money.ExchangeRateQuote{}, err
	}
	rateValue, ok := p.rates[rateKey(base, target)]
	if !ok {
		return money.ExchangeRateQuote{}, fmt.Errorf("%w: %s/%s", money.ErrUnsupportedExchangeRate, base, target)
	}
	rate, err := money.NewExchangeRate(base, target, rateValue)
	if err != nil {
		return money.ExchangeRateQuote{}, err
	}
	now := p.now()
	return money.ExchangeRateQuote{
		Rate:       rate,
		Source:     money.ECBSource + " demo snapshot",
		ObservedAt: now.Add(-6 * time.Hour),
		FetchedAt:  now,
		ExpiresAt:  now.Add(18 * time.Hour),
	}, nil
}

func rateKey(base, target money.Currency) string {
	return base.Code() + "/" + target.Code()
}
