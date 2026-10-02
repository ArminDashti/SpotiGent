package store

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChatMessage is one turn of a persisted AI conversation.
type ChatMessage struct {
	Role    string    `json:"role"` // "user" | "assistant"
	Content string    `json:"content"`
	At      time.Time `json:"at"`
}

// ChatSession is a named conversation persisted across restarts.
type ChatSession struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Messages  []ChatMessage `json:"messages"`
}

// ChatStore persists AI chat sessions in a JSON file (data/chats.json).
type ChatStore struct {
	mu   sync.RWMutex
	path string
	// sessions are kept oldest-first in memory; newest activity last.
	sessions []ChatSession
	loaded   bool
}

// NewChatStore creates a ChatStore backed by path (directories created).
func NewChatStore(path string) (*ChatStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &ChatStore{path: path}, nil
}

// loadLocked reads the file once into memory (call with lock held).
func (c *ChatStore) loadLocked() error {
	if c.loaded {
		return nil
	}
	c.loaded = true
	raw, err := os.ReadFile(c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(raw, &c.sessions)
}

// saveLocked writes all sessions to disk (call with write lock held).
func (c *ChatStore) saveLocked() error {
	raw, err := json.MarshalIndent(c.sessions, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// List returns sessions sorted by most recent activity first.
// Messages are omitted from the summary.
func (c *ChatStore) List() []ChatSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.loadLocked(); err != nil {
		return []ChatSession{}
	}
	out := make([]ChatSession, 0, len(c.sessions))
	for _, s := range c.sessions {
		s.Messages = nil
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// Get returns a full session by ID.
func (c *ChatStore) Get(id string) (ChatSession, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.loadLocked(); err != nil {
		return ChatSession{}, false
	}
	for _, s := range c.sessions {
		if s.ID == id {
			return s, true
		}
	}
	return ChatSession{}, false
}

// Append creates a session when id is empty, otherwise appends the
// messages to the existing session. The session title is derived from
// the first user message when missing. Returns the stored session.
func (c *ChatStore) Append(id string, messages ...ChatMessage) (ChatSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.loadLocked(); err != nil {
		return ChatSession{}, err
	}

	idx := -1
	if id != "" {
		for i := range c.sessions {
			if c.sessions[i].ID == id {
				idx = i
				break
			}
		}
	}

	now := time.Now()
	if idx < 0 {
		sess := ChatSession{
			ID:        newChatID(),
			Title:     titleFrom(messages),
			CreatedAt: now,
			UpdatedAt: now,
			Messages:  append([]ChatMessage{}, messages...),
		}
		c.sessions = append(c.sessions, sess)
		if err := c.saveLocked(); err != nil {
			return ChatSession{}, err
		}
		return sess, nil
	}

	sess := &c.sessions[idx]
	sess.Messages = append(sess.Messages, messages...)
	sess.UpdatedAt = now
	if sess.Title == "" {
		sess.Title = titleFrom(sess.Messages)
	}
	if err := c.saveLocked(); err != nil {
		return ChatSession{}, err
	}
	return *sess, nil
}

// Delete removes a session by ID.
func (c *ChatStore) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.loadLocked(); err != nil {
		return err
	}
	for i := range c.sessions {
		if c.sessions[i].ID == id {
			c.sessions = append(c.sessions[:i], c.sessions[i+1:]...)
			return c.saveLocked()
		}
	}
	return os.ErrNotExist
}

// titleFrom derives a short session title from the first user message.
func titleFrom(msgs []ChatMessage) string {
	for _, m := range msgs {
		if m.Role == "user" {
			t := strings.TrimSpace(strings.ReplaceAll(m.Content, "\n", " "))
			runes := []rune(t)
			if len(runes) > 60 {
				return string(runes[:60]) + "…"
			}
			return t
		}
	}
	return "New chat"
}

// newChatID returns a sortable unique session ID.
func newChatID() string {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	const hex = "0123456789abcdef"
	out := make([]byte, 16)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}
