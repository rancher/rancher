package moq

import (
	"fmt"
	"go/format"

	"golang.org/x/tools/imports"
)

func goimports(src []byte) ([]byte, error) {
	formatted, err := imports.Process("filename", src, &imports.Options{
		TabWidth:  8,
		TabIndent: true,
		Comments:  true,
		Fragment:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("goimports: %s", err)
	}

	return formatted, nil
}

func gofmt(src []byte) ([]byte, error) {
	formatted, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("go/format: %s", err)
	}

	return formatted, nil
}
