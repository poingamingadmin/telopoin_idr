package helpers

import (
	"math/rand"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func randomLetters(n int) string {
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[src.Intn(len(letterBytes))]
	}
	return string(b)
}

func GenerateAgentCode() string {
	return "0" + randomLetters(3)
}

func AbsInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
