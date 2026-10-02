package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestChatStoreAppendListGetDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chats.json")
	cs, err := NewChatStore(path)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}

	// Create a session (empty ID) and derive its title from the first user message.
	sess, err := cs.Append("", ChatMessage{
		Role: "user", Content: "Create a chill lo-fi playlist with 15 tracks", At: time.Now(),
	})
	if err != nil {
		t.Fatalf("Append create: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected a generated session ID")
	}
	if sess.Title != "Create a chill lo-fi playlist with 15 tracks" {
		t.Fatalf("title = %q", sess.Title)
	}

	// Append to the same session.
	if _, err := cs.Append(sess.ID,
		ChatMessage{Role: "user", Content: "thanks", At: time.Now()},
		ChatMessage{Role: "assistant", Content: "done", At: time.Now()},
	); err != nil {
		t.Fatalf("Append follow-up: %v", err)
	}

	got, ok := cs.Get(sess.ID)
	if !ok {
		t.Fatal("Get: session not found")
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(got.Messages))
	}

	// List summaries omit messages but sort newest activity first.
	list := cs.List()
	if len(list) != 1 {
		t.Fatalf("List = %d sessions, want 1", len(list))
	}
	if list[0].Messages != nil {
		t.Fatal("List should omit messages")
	}

	// Persistence: a fresh store on the same path sees the session.
	cs2, err := NewChatStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if _, ok := cs2.Get(sess.ID); !ok {
		t.Fatal("session not persisted to disk")
	}

	if err := cs2.Delete(sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := cs2.Get(sess.ID); ok {
		t.Fatal("session still present after delete")
	}
	if err := cs2.Delete("missing"); err == nil {
		t.Fatal("Delete of missing ID should fail")
	}
}

func TestChatStoreLongTitleTruncated(t *testing.T) {
	cs, err := NewChatStore(filepath.Join(t.TempDir(), "chats.json"))
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "x"
	}
	sess, err := cs.Append("", ChatMessage{Role: "user", Content: long, At: time.Now()})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	runes := []rune(sess.Title)
	if len(runes) > 61 {
		t.Fatalf("title not truncated: %d runes", len(runes))
	}
}
