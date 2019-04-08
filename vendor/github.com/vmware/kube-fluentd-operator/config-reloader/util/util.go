// Copyright © 2018 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause

package util

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os/exec"
	"sort"
	"strings"
	"unicode"
)

const (
	maskFile = 0664
)

func Trim(s string) string {
	return strings.TrimFunc(s, unicode.IsSpace)
}

func MakeFluentdSafeName(s string) string {
	buf := &bytes.Buffer{}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			buf.WriteRune('-')
		} else {
			buf.WriteRune(r)
		}
	}

	return buf.String()
}

func ToRubyMapLiteral(labels map[string]string) string {
	if len(labels) == 0 {
		return "{}"
	}

	buf := &bytes.Buffer{}
	buf.WriteString("{")
	for _, k := range SortedKeys(labels) {
		fmt.Fprintf(buf, "'%s'=>'%s',", k, labels[k])
	}
	buf.Truncate(buf.Len() - 1)
	buf.WriteString("}")

	return buf.String()
}

func Hash(owner string, value string) string {
	h := sha1.New()

	h.Write([]byte(owner))
	h.Write([]byte(":"))
	h.Write([]byte(value))

	b := h.Sum(nil)
	return hex.EncodeToString(b[:])
}

func SortedKeys(m map[string]string) []string {
	keys := make([]string, len(m))
	i := 0

	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	return keys
}

func ExecAndGetOutput(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()

	return string(out), err
}

func WriteStringToFile(filename string, data string) error {
	return ioutil.WriteFile(filename, []byte(data), maskFile)
}

func TrimTrailingComment(line string) string {
	i := strings.IndexByte(line, '#')
	if i > 0 {
		line = Trim(line[0:i])
	} else {
		line = Trim(line)
	}

	return line
}
