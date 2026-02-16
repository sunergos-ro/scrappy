package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
)

func TestReadWriteRPCMessageRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "ping",
	}

	if err := writeRPCMessage(writer, payload); err != nil {
		t.Fatalf("writeRPCMessage failed: %v", err)
	}

	reader := bufio.NewReader(&buf)
	raw, err := readRPCMessage(reader)
	if err != nil {
		t.Fatalf("readRPCMessage failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if got := decoded["method"]; got != "ping" {
		t.Fatalf("unexpected method: %#v", got)
	}
}

func TestReadRPCMessageRequiresContentLength(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReader(bytes.NewBufferString("X-Test: true\r\n\r\n{}"))
	if _, err := readRPCMessage(reader); err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}
