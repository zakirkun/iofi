package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestValidSignature(t *testing.T) {
	body := []byte(`{"action":"opened"}`)

	if !validSignature("s3cret", body, sign("s3cret", body)) {
		t.Fatal("a correctly signed delivery was rejected")
	}
	if validSignature("s3cret", body, sign("wrong", body)) {
		t.Fatal("a delivery signed with the wrong secret was accepted")
	}
	if validSignature("s3cret", body, "") {
		t.Fatal("an unsigned delivery was accepted while a secret is configured")
	}
	if validSignature("s3cret", []byte(`{"action":"closed"}`), sign("s3cret", body)) {
		t.Fatal("a tampered body was accepted")
	}
	// No secret is not an opt-out. An unverified webhook can create tasks and
	// move the board, so it must be refused unless opted out explicitly.
	if validSignature("", body, "") {
		t.Fatal("a delivery with no configured secret must be rejected")
	}
	if validSignature("", body, sign("s3cret", body)) {
		t.Fatal("a signature cannot be valid when no secret is configured")
	}
	if !validSignature(OptOut, body, "") {
		t.Fatal("the explicit opt-out value should accept unsigned deliveries")
	}
}

// The loop guard is only a guard if the hash actually changes with the state.
func TestStateHashDistinguishesState(t *testing.T) {
	a := stateHash("title", "spec", "in_progress", "open")
	if a != stateHash("title", "spec", "in_progress", "open") {
		t.Fatal("hash is not stable for identical state")
	}
	if a == stateHash("title", "spec", "done", "closed") {
		t.Fatal("a column change must change the hash, or inbound echoes get dropped forever")
	}
	// Field boundaries must not be ambiguous.
	if stateHash("ab", "c") == stateHash("a", "bc") {
		t.Fatal("hash inputs are not delimited")
	}
}

func TestColumnFromLabels(t *testing.T) {
	got := columnFromLabels([]Label{{Name: "bug"}, {Name: "iofi:review"}})
	if got != "review" {
		t.Fatalf("got %q", got)
	}
	if columnFromLabels([]Label{{Name: "bug"}}) != "" {
		t.Fatal("a repo label must not be read as a column")
	}
	if names := labelNames([]Label{{Name: "bug"}, {Name: "iofi:review"}}); len(names) != 1 || names[0] != "bug" {
		t.Fatalf("iofi labels must not round-trip into task labels: %v", names)
	}
}
