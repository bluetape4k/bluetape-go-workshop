// Package main 은 다국어 언어 라우팅 미리보기를 출력한다.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/bluetape4k/bluetape-go-workshop/examples/multilingual-language-routing/internal/routing"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("multilingual-language-routing", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: multilingual-language-routing [--preload]")
		flags.PrintDefaults()
	}
	preload := flags.Bool("preload", false, "preload selected language models during router construction")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "unexpected argument: %q\n", flags.Arg(0))
		return 2
	}

	preview, err := routing.NewPreview(*preload)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build multilingual language routing preview: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(preview); err != nil {
		_, _ = fmt.Fprintf(stderr, "encode multilingual language routing preview: %v\n", err)
		return 1
	}
	return 0
}
