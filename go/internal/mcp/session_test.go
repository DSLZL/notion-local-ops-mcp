package mcp

import "testing"

func TestSessionPreservesRequestMetadata(t *testing.T) {
	session := NewSession(Request{
		ID:     "req-1",
		Method: "tools/list",
	})

	if session.Method() != "tools/list" {
		t.Fatalf("session method = %q, want tools/list", session.Method())
	}

	resp := session.Response(map[string]any{"ok": true})
	if resp.ID != "req-1" {
		t.Fatalf("response ID = %v, want req-1", resp.ID)
	}
	if resp.Result["ok"] != true {
		t.Fatal("response result must preserve payload")
	}
}
