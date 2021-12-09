package namegenerator

import (
	"math/rand"
	"time"
)

const lowerLetterBytes = "abcdefghijklmnopqrstuvwxyz"
const upperLetterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const numberBytes = "0123456789"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandStringLower(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lowerLetterBytes[rand.Intn(len(lowerLetterBytes))]
	}
	return string(b)
}

func RandStringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func RandStringAll(length int) string {
	return RandStringWithCharset(length, lowerLetterBytes+upperLetterBytes+numberBytes)
}
