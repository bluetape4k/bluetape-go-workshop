// Command account-migration-checkpoint-restart 는 local checkpoint demo를 실행한다.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bluetape4k/bluetape-go-workshop/examples/account-migration-checkpoint-restart/internal/accountmigration"
)

func main() {
	result, err := accountmigration.RunDemo(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "run account migration checkpoint restart demo: %v\n", err)
		os.Exit(1)
	}
	output, err := accountmigration.MarshalResult(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal account migration checkpoint restart demo: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}
