package sessionstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var ErrInvalidSessionID = errors.New("invalid session id")

var sessionIDPattern = regexp.MustCompile(`^session-[0-9]+$`)

type FSStore struct {
	root string
	mu   sync.RWMutex
}

func NewFSStore(root string) *FSStore {
	return &FSStore{root: root}
}

func (s *FSStore) Create(input SessionInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339Nano)
	session := Session{
		ID:        sessionID,
		Shell:     input.Shell,
		CWD:       input.CWD,
		Status:    "running",
		PID:       input.PID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	sessionDir := filepath.Join(s.root, "sessions", sessionID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return Session{}, err
	}
	if err := s.writeSession(sessionID, session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *FSStore) Get(sessionID string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getUnlocked(sessionID)
}

func (s *FSStore) getUnlocked(sessionID string) (Session, error) {
	metaPath, err := s.metaPath(sessionID)
	if err != nil {
		return Session{}, err
	}

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return Session{}, err
	}

	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *FSStore) Update(sessionID string, session Session) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.ID = sessionID
	if session.CreatedAt == "" {
		if existing, err := s.getUnlocked(sessionID); err == nil {
			session.CreatedAt = existing.CreatedAt
		}
	}
	if session.UpdatedAt == "" {
		session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := s.writeSession(sessionID, session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *FSStore) AppendOutput(sessionID, chunk string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	outputPath, err := s.outputPath(sessionID)
	if err != nil {
		return 0, err
	}

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	if _, err := file.WriteString(chunk); err != nil {
		return 0, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	session, err := s.getUnlocked(sessionID)
	if err != nil {
		return info.Size(), err
	}
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.writeSession(sessionID, session); err != nil {
		return info.Size(), err
	}

	return info.Size(), nil
}

func (s *FSStore) ReadOutput(sessionID string, offset, limit int64) (OutputRead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	outputPath, err := s.outputPath(sessionID)
	if err != nil {
		return OutputRead{}, err
	}

	file, err := os.Open(outputPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return OutputRead{Offset: offset, NextOffset: offset}, nil
		}
		return OutputRead{}, err
	}
	defer file.Close()

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 4096
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return OutputRead{}, err
	}

	buf := make([]byte, limit)
	n, err := file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return OutputRead{}, err
	}

	nextOffset := offset + int64(n)
	info, statErr := file.Stat()
	truncated := false
	if statErr == nil && nextOffset < info.Size() {
		truncated = true
	}

	return OutputRead{
		Content:    string(buf[:n]),
		Offset:     offset,
		NextOffset: nextOffset,
		Truncated:  truncated,
	}, nil
}

func (s *FSStore) writeSession(sessionID string, session Session) error {
	metaPath, err := s.metaPath(sessionID)
	if err != nil {
		return err
	}

	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, metaPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *FSStore) metaPath(sessionID string) (string, error) {
	if !sessionIDPattern.MatchString(sessionID) {
		return "", ErrInvalidSessionID
	}
	return filepath.Join(s.root, "sessions", sessionID, "meta.json"), nil
}

func (s *FSStore) outputPath(sessionID string) (string, error) {
	if !sessionIDPattern.MatchString(sessionID) {
		return "", ErrInvalidSessionID
	}
	return filepath.Join(s.root, "sessions", sessionID, "output.log"), nil
}
