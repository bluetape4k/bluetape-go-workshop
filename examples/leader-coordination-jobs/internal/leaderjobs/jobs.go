// Package leaderjobs 는 백엔드 작업에서 Redis 리더 선출을 활용하는 방법을 보여준다.
package leaderjobs

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

// MigrationGate 는 현재 인스턴스가 리더가 된 뒤에만 일회성 작업을 실행한다.
func MigrationGate(ctx context.Context, elector leader.Elector, migrate func(context.Context) error) error {
	if err := elector.Campaign(ctx); err != nil {
		if errors.Is(err, leader.ErrAlreadyLeader) {
			return migrate(ctx)
		}
		return err
	}
	defer func() {
		_ = elector.Resign(context.Background())
	}()
	return migrate(ctx)
}

// CacheWarmer 는 리더십을 보유하는 동안 주기 작업을 실행한다.
type CacheWarmer struct {
	elector leader.Elector
	warm    func(context.Context) error
	count   atomic.Int32
}

// NewCacheWarmer 는 리더가 소유하는 캐시 워머를 생성한다.
func NewCacheWarmer(elector leader.Elector, warm func(context.Context) error) *CacheWarmer {
	return &CacheWarmer{elector: elector, warm: warm}
}

// Runs 는 완료된 워밍 사이클 수를 반환한다.
func (w *CacheWarmer) Runs() int {
	return int(w.count.Load())
}

// Run 은 워머를 시작하고 취소, 사임, 리더십 상실 시 중단한다.
func (w *CacheWarmer) Run(ctx context.Context, interval time.Duration) error {
	if err := w.elector.Campaign(ctx); err != nil && !errors.Is(err, leader.ErrAlreadyLeader) {
		return err
	}
	defer func() {
		_ = w.elector.Resign(context.Background())
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if !w.elector.IsLeader() {
			return leader.ErrNotLeader
		}
		if err := w.warm(ctx); err != nil {
			return err
		}
		w.count.Add(1)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
