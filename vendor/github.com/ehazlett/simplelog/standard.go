package simplelog

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type StandardFormatter struct{}

func (s *StandardFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}
	// 2015/09/28 17:06:03
	t := time.Now().Local()
	timestamp := fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d",
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
	)
	msg := fmt.Sprintf("%s [%s] %s", timestamp, strings.ToUpper(entry.Level.String()), entry.Message)
	fmt.Fprintf(b, msg)
	b.WriteByte('\n')
	return b.Bytes(), nil

}
