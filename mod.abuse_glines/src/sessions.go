package abuse_glines

import (
	"net/http"
	"sync"

	"github.com/gorilla/sessions"
	gorillaSessions "github.com/gorilla/sessions"
)

// InMemoryStore is a custom in-memory session store
type InMemoryStore struct {
	data map[string]*gorillaSessions.Session
	mu   sync.RWMutex
}

// NewInMemoryStore creates a new instance of InMemoryStore
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]*sessions.Session),
	}
}

// New creates a new session
func (s *InMemoryStore) New(r *http.Request, name string) (*gorillaSessions.Session, error) {
	session := gorillaSessions.NewSession(s, name)
	session.Options = &gorillaSessions.Options{
		Path:   "/",
		MaxAge: 86400,
	}
	return session, nil
}

// Get fetches a session from the in-memory store
func (s *InMemoryStore) Get(r *http.Request, name string) (*gorillaSessions.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session, exists := s.data[name]; exists {
		return session, nil
	}
	// If session doesn't exist, create a new one
	session := sessions.NewSession(s, name)
	session.Options = &sessions.Options{
		Path:   "/",
		MaxAge: 86400,
	}
	return session, nil
}

// Save stores the session in the in-memory map
func (s *InMemoryStore) Save(r *http.Request, w http.ResponseWriter, session *gorillaSessions.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[session.Name()] = session
	return nil
}
