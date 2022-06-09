package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input  string
	expect string
}

func newTestCase(input string, expect string) testCase {
	return testCase{
		input,
		expect,
	}
}
func TestEscapeCommas(t *testing.T) {
	assert := assert.New(t)

	invalidArguments := []testCase{
		newTestCase("asdf", "asdf"), // no commas, so nothing should be escaped
		newTestCase("{asdf", "{asdf"),
		newTestCase("asdf{", "asdf{"),
		newTestCase("asdf}", "asdf}"),
		newTestCase("asd}asdf", "asd}asdf"),
		newTestCase("asd{asdf}dasf", "asd{asdf}dasf"),
		newTestCase("as,df", "as\\,df"), // value not a list so commas should be escaped
		newTestCase("asd,f{", "asd\\,f{"),
		newTestCase("as,df}", "as\\,df}"),
		newTestCase("asd}as,df", "asd}as\\,df"),
		newTestCase("asd{asdf}da,sf", "asd{asdf}da\\,sf"),
		newTestCase(",asdf", "\\,asdf"),
		newTestCase("{,asdf", "{,asdf"), // helm would recognize as a list, commas should not be escaped
		newTestCase("{as,df}", "{as,df}"),
		newTestCase("{{", "{{"),
		newTestCase("", ""),
	}

	for _, arg := range invalidArguments {
		result := escapeCommas(arg.input)
		assert.Equal(arg.expect, result)
	}
}
