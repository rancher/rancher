package simplelog

import (
	"bytes"
	"fmt"

	"github.com/sirupsen/logrus"
)

type SimpleFormatter struct{}

func (s *SimpleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}
	fmt.Fprintf(b, entry.Message)
	b.WriteByte('\n')
	return b.Bytes(), nil
}
