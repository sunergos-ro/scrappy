package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func newID() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func buildObjectKey(prefix string, format string) string {
	cleanFormat := strings.ToLower(strings.TrimSpace(format))
	if cleanFormat == "" {
		cleanFormat = "jpeg"
	}
	now := time.Now().UTC()
	return fmt.Sprintf("%s/%04d/%02d/%02d/%s.%s", strings.TrimSuffix(prefix, "/"), now.Year(), now.Month(), now.Day(), newID(), cleanFormat)
}
