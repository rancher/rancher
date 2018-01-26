package parse

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractCatalogBlock(t *testing.T) {
	assert.Equal(t, extractCatalogBlock(string(`
s1:
  image: nginx
.catalog:
  questions:
  - q1
  - q2`)), string(`.catalog:
  questions:
  - q1
  - q2`))

	assert.Equal(t, extractCatalogBlock(string(`
.catalog:
  questions:
  - q1
  - q2
s1:
  image: nginx`)), string(`.catalog:
  questions:
  - q1
  - q2`))

	assert.Equal(t, extractCatalogBlock(string(`
catalog:
  questions:
  - q1
  - q2
services:
  s1:
    image: nginx`)), string(`catalog:
  questions:
  - q1
  - q2`))

	assert.Equal(t, extractCatalogBlock(string(`
services:
  s1:
    image: nginx
catalog:
  questions:
  - q1
  - q2`)), string(`catalog:
  questions:
  - q1
  - q2`))
}
