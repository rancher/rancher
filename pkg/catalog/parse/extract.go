package parse

import (
	"strings"
	"unicode"
)

func extractCatalogBlock(contents string) string {
	var catalogBlock []string
	inBlock := false
	lines := strings.Split(contents, "\n")
	for i, line := range lines {
		if isCommentLine(line) {
			continue
		}

		if strings.HasPrefix(line, "catalog:") || strings.HasPrefix(line, ".catalog:") {
			inBlock = true
		}

		if inBlock {
			if i == len(lines)-1 {
				catalogBlock = append(catalogBlock, line)
				return strings.Join(catalogBlock, "\n")
			}
			if len(catalogBlock) > 1 && len(line) > 0 && !unicode.IsSpace(rune(line[0])) {
				return strings.Join(catalogBlock, "\n")
			}
			catalogBlock = append(catalogBlock, line)
		}
	}
	return ""
}

func isCommentLine(line string) bool {
	line = strings.TrimLeft(line, " \t")
	return len(line) > 0 && line[0] == '#'
}
