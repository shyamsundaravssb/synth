package ipc

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestNewRequest_PingRoundTrip(t *testing.T) {
	req, err := NewRequest(TypePing, PingPayload{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Type != TypePing {
		t.Errorf("expected Type %q, got %q", TypePing, req.Type)
	}
	if string(req.Payload) != "{}" {
		t.Errorf("expected payload {}, got %s", string(req.Payload))
	}
}

func TestNewRequest_SearchRoundTrip(t *testing.T) {
	payload := SearchPayload{
		Query:     "test",
		Limit:     10,
		FilePath:  "main.go",
		Since:     "yesterday",
		Developer: "alice",
	}
	req, err := NewRequest(TypeSearch, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := ParseSearchPayload(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Query != payload.Query || parsed.Limit != payload.Limit || parsed.FilePath != payload.FilePath || parsed.Since != payload.Since || parsed.Developer != payload.Developer {
		t.Errorf("parsed payload does not match original")
	}
}

func TestNewOKResponse_RoundTrip(t *testing.T) {
	data := PingData{PID: 123, Version: "0.1.0"}
	resp, err := NewOKResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != StatusOK {
		t.Errorf("expected Status %q, got %q", StatusOK, resp.Status)
	}
	parsed, err := ParsePingData(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.PID != data.PID || parsed.Version != data.Version {
		t.Errorf("parsed data does not match original")
	}
}

func TestNewErrorResponse_Structure(t *testing.T) {
	resp := NewErrorResponse("something failed", "ERR_001")
	if resp.Status != StatusError {
		t.Errorf("expected Status %q, got %q", StatusError, resp.Status)
	}
	parsed, err := ParseErrorData(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Message != "something failed" || parsed.Code != "ERR_001" {
		t.Errorf("parsed error data does not match original")
	}
}

func TestEncodeDecodeRequest_RoundTrip(t *testing.T) {
	req, _ := NewRequest(TypePing, PingPayload{})
	var buf bytes.Buffer
	if err := EncodeRequest(&buf, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte{'\n'}) {
		t.Errorf("expected encoded bytes to end with newline")
	}
	decoded, err := DecodeRequest(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Type != req.Type {
		t.Errorf("decoded Type does not match")
	}
}

func TestEncodeDecodeResponse_RoundTrip(t *testing.T) {
	resp, _ := NewOKResponse(PingData{})
	var buf bytes.Buffer
	if err := EncodeResponse(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte{'\n'}) {
		t.Errorf("expected encoded bytes to end with newline")
	}
	decoded, err := DecodeResponse(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Status != resp.Status {
		t.Errorf("decoded Status does not match")
	}
}

func TestDecodeRequest_InvalidJSON(t *testing.T) {
	r := strings.NewReader("notjson\n")
	_, err := DecodeRequest(r)
	if err == nil {
		t.Errorf("expected error decoding invalid JSON")
	}
}

func TestDecodeResponse_InvalidJSON(t *testing.T) {
	r := strings.NewReader("notjson\n")
	_, err := DecodeResponse(r)
	if err == nil {
		t.Errorf("expected error decoding invalid JSON")
	}
}

func TestSearchPayload_OptionalFields(t *testing.T) {
	payload := SearchPayload{
		Query: "test",
		Limit: 10,
	}
	req, _ := NewRequest(TypeSearch, payload)
	s := string(req.Payload)
	if strings.Contains(s, "\"file\"") || strings.Contains(s, "\"since\"") || strings.Contains(s, "\"developer\"") {
		t.Errorf("expected omitempty fields to be absent, got %s", s)
	}
}

func TestSearchResultItem_ScoreField(t *testing.T) {
	item := SearchResultItem{Score: 0.87}
	resp, _ := NewOKResponse(item)
	s := string(resp.Data)
	if !strings.Contains(s, "\"score\":0.87") {
		t.Errorf("expected score field in output, got %s", s)
	}
}

func TestEncodeRequest_MultipleMessages(t *testing.T) {
	req1, _ := NewRequest(TypePing, PingPayload{})
	req2, _ := NewRequest(TypeStatus, StatusPayload{})
	req3, _ := NewRequest(TypeSearch, SearchPayload{})

	var buf bytes.Buffer
	EncodeRequest(&buf, req1)
	EncodeRequest(&buf, req2)
	EncodeRequest(&buf, req3)

	if bytes.Count(buf.Bytes(), []byte{'\n'}) != 3 {
		t.Errorf("expected exactly 3 newlines")
	}

	importBufio := bufio.NewReader(&buf)

	dec1, err := DecodeRequest(importBufio)
	if err != nil || dec1.Type != TypePing {
		t.Errorf("failed to decode message 1")
	}
	dec2, err := DecodeRequest(importBufio)
	if err != nil || dec2.Type != TypeStatus {
		t.Errorf("failed to decode message 2")
	}
	dec3, err := DecodeRequest(importBufio)
	if err != nil || dec3.Type != TypeSearch {
		t.Errorf("failed to decode message 3")
	}
}
