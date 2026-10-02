package logs

import (
	"strings"
	"testing"
)

func TestMaskingRegisteredSecrets(t *testing.T) {
	l := New(100)
	l.RegisterSecret("super-secret-value", "sk-or-v1-abcdef123456")

	msg := l.Mask("failed with super-secret-value and sk-or-v1-abcdef123456")
	if strings.Contains(msg, "super-secret-value") {
		t.Fatalf("secret leaked: %q", msg)
	}
	if want := strings.Repeat("*", len("super-secret-value")); !strings.Contains(msg, want) {
		t.Fatalf("expected mask of actual length %d in %q", len("super-secret-value"), msg)
	}
	if want := strings.Repeat("*", len("sk-or-v1-abcdef123456")); !strings.Contains(msg, want) {
		t.Fatalf("expected mask of actual length %d in %q", len("sk-or-v1-abcdef123456"), msg)
	}
}

func TestRingBufferAndLevelFilter(t *testing.T) {
	l := New(100)
	l.Info("started")
	l.Warn("careful")
	l.Error("boom")

	if got := l.All(""); len(got) != 3 {
		t.Fatalf("All = %d, want 3", len(got))
	}
	// Newest first.
	if got := l.All(""); got[0].Message != "boom" {
		t.Fatalf("newest first violated: %q", got[0].Message)
	}
	if got := l.All(Error); len(got) != 1 || got[0].Level != Error {
		t.Fatalf("Error filter = %+v", got)
	}
	if got := l.All(Warning); len(got) != 1 || got[0].Level != Warning {
		t.Fatalf("Warning filter = %+v", got)
	}
	if got := l.All(Info); len(got) != 1 || got[0].Level != Info {
		t.Fatalf("Info filter = %+v", got)
	}

	l.Clear()
	if got := l.All(""); len(got) != 0 {
		t.Fatalf("Clear failed: %d entries", len(got))
	}
}

func TestRingBufferEvictsOldest(t *testing.T) {
	l := New(100)
	for i := 0; i < 150; i++ {
		l.Info("entry %d", i)
	}
	got := l.All("")
	if len(got) != 100 {
		t.Fatalf("len = %d, want 100", len(got))
	}
	if got[0].Message != "entry 149" {
		t.Fatalf("newest = %q", got[0].Message)
	}
	if got[len(got)-1].Message != "entry 50" {
		t.Fatalf("oldest = %q", got[len(got)-1].Message)
	}
}
