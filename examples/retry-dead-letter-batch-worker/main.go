// Command retry-dead-letter-batch-worker 는 로컬 배치 재시도 데모를 실행한다.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bluetape4k/bluetape-go-workshop/examples/retry-dead-letter-batch-worker/internal/ticketworker"
)

func main() {
	result, err := ticketworker.RunDemo(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "retry dead-letter batch worker failed: %v\n", err)
		os.Exit(1)
	}

	output, err := ticketworker.MarshalResult(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal retry dead-letter batch worker result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}
