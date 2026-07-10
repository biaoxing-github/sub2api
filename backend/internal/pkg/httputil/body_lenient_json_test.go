package httputil

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tidwall/gjson"
)

func TestNormalizeLenientJSONRequestBodyAcceptsControlCharsInStrings(t *testing.T) {
	body := []byte("{\"messages\":[{\"content\":\"hello\x00world\"}]}")
	if gjson.ValidBytes(body) {
		t.Fatalf("strict JSON should reject raw control bytes")
	}

	got, err := NormalizeLenientJSONRequestBody(body, 1024)
	if err != nil {
		t.Fatalf("NormalizeLenientJSONRequestBody: %v", err)
	}
	if !gjson.ValidBytes(got) {
		t.Fatalf("normalized body should be valid JSON: %q", got)
	}
	if gotRaw := gjson.GetBytes(got, "messages.0.content").Raw; gotRaw != `"hello\u0000world"` {
		t.Fatalf("raw value mismatch: got %q", gotRaw)
	}
}

func TestNormalizeLenientJSONRequestBodyRejectsExpansionPastLimit(t *testing.T) {
	body := []byte("{\"input\":\"\x00\x00\"}")

	_, err := NormalizeLenientJSONRequestBody(body, int64(len(body)+5))

	var maxErr *http.MaxBytesError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected MaxBytesError, got %T %v", err, err)
	}
}

func TestReadLenientJSONRequestBodyWithPreallocHonorsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ReadLenientJSONRequestBodyWithPrealloc(r, 32)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		if !gjson.ValidBytes(body) {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader([]byte("{\"input\":\"\x00\"}")))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status mismatch: got %d", resp.StatusCode)
	}
}
