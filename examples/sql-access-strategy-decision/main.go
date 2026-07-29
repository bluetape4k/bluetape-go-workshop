// Package main 은 SQL 접근 전략 결정 예제를 출력한다.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/sql-access-strategy-decision/internal/sqlstrategy"
)

func main() {
	report, err := sqlstrategy.NewDecisionReport(sqlstrategy.Hold{
		ID:       "hold-1001",
		Customer: "customer-42",
		Status:   sqlstrategy.StatusPending,
	})
	if err != nil {
		log.Fatalf("build report: %v", err)
	}

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("encode report: %v", err)
	}
	fmt.Println(string(encoded))
}
