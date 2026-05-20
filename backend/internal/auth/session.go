package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"nestify/backend/internal/model"
)

type Session struct {
	Token     string
	User      model.SessionUser
	ExpiresAt time.Time
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]Session),
		ttl:      ttl,
	}
}

func (m *SessionManager) Create(user model.SessionUser) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}

	session := Session{
		Token:     token,
		User:      user,
		ExpiresAt: time.Now().UTC().Add(m.ttl),
	}

	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()

	return session, nil
}

func (m *SessionManager) Get(token string) (Session, bool) {
	m.mu.RLock()
	session, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return Session{}, false
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		m.Delete(token)
		return Session{}, false
	}

	return session, true
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
