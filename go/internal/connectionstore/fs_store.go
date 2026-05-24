package connectionstore

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

var ErrInvalidConnectionID = errors.New("invalid connection id")

var connectionIDPattern = regexp.MustCompile(`^conn-[0-9]+$`)

type FSStore struct {
	root string
	mu   sync.RWMutex
}

func NewFSStore(root string) *FSStore {
	return &FSStore{root: root}
}

func (s *FSStore) Create(input ConnectionInput) (Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connectionID := fmt.Sprintf("conn-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339Nano)
	connection := Connection{
		ID:         connectionID,
		Network:    input.Network,
		Host:       input.Host,
		Port:       input.Port,
		RemoteAddr: input.RemoteAddr,
		Status:     "open",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if connection.Network == "" {
		connection.Network = "tcp"
	}

	connectionDir := filepath.Join(s.root, "connections", connectionID)
	if err := os.MkdirAll(connectionDir, 0o700); err != nil {
		return Connection{}, err
	}
	if err := s.writeConnection(connectionID, connection); err != nil {
		return Connection{}, err
	}

	return connection, nil
}

func (s *FSStore) Get(connectionID string) (Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getUnlocked(connectionID)
}

func (s *FSStore) getUnlocked(connectionID string) (Connection, error) {
	metaPath, err := s.metaPath(connectionID)
	if err != nil {
		return Connection{}, err
	}
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return Connection{}, err
	}

	var connection Connection
	if err := json.Unmarshal(raw, &connection); err != nil {
		return Connection{}, err
	}
	return connection, nil
}

func (s *FSStore) Update(connectionID string, connection Connection) (Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connection.ID = connectionID
	if connection.CreatedAt == "" {
		if existing, err := s.getUnlocked(connectionID); err == nil {
			connection.CreatedAt = existing.CreatedAt
		}
	}
	if connection.UpdatedAt == "" {
		connection.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := s.writeConnection(connectionID, connection); err != nil {
		return Connection{}, err
	}
	return connection, nil
}

func (s *FSStore) AppendInput(connectionID, chunk string) (int64, error) {
	return s.appendLog(connectionID, chunk, s.inputPath)
}

func (s *FSStore) ReadInput(connectionID string, offset, limit int64) (LogRead, error) {
	return s.readLog(connectionID, offset, limit, s.inputPath)
}

func (s *FSStore) AppendOutput(connectionID, chunk string) (int64, error) {
	return s.appendLog(connectionID, chunk, s.outputPath)
}

func (s *FSStore) ReadOutput(connectionID string, offset, limit int64) (LogRead, error) {
	return s.readLog(connectionID, offset, limit, s.outputPath)
}

func (s *FSStore) appendLog(connectionID, chunk string, pathFn func(string) (string, error)) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logPath, err := pathFn(connectionID)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return 0, err
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
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

	connection, err := s.getUnlocked(connectionID)
	if err != nil {
		return info.Size(), err
	}
	connection.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.writeConnection(connectionID, connection); err != nil {
		return info.Size(), err
	}

	return info.Size(), nil
}

func (s *FSStore) readLog(connectionID string, offset, limit int64, pathFn func(string) (string, error)) (LogRead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logPath, err := pathFn(connectionID)
	if err != nil {
		return LogRead{}, err
	}

	file, err := os.Open(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LogRead{Offset: offset, NextOffset: offset}, nil
		}
		return LogRead{}, err
	}
	defer file.Close()

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 4096
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return LogRead{}, err
	}

	buf := make([]byte, limit)
	n, err := file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return LogRead{}, err
	}

	nextOffset := offset + int64(n)
	info, statErr := file.Stat()
	truncated := false
	if statErr == nil && nextOffset < info.Size() {
		truncated = true
	}

	return LogRead{
		Content:    string(buf[:n]),
		Offset:     offset,
		NextOffset: nextOffset,
		Truncated:  truncated,
	}, nil
}

func (s *FSStore) writeConnection(connectionID string, connection Connection) error {
	metaPath, err := s.metaPath(connectionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o700); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(connection, "", "  ")
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

func (s *FSStore) metaPath(connectionID string) (string, error) {
	if !connectionIDPattern.MatchString(connectionID) {
		return "", ErrInvalidConnectionID
	}
	return filepath.Join(s.root, "connections", connectionID, "meta.json"), nil
}

func (s *FSStore) inputPath(connectionID string) (string, error) {
	if !connectionIDPattern.MatchString(connectionID) {
		return "", ErrInvalidConnectionID
	}
	return filepath.Join(s.root, "connections", connectionID, "input.log"), nil
}

func (s *FSStore) outputPath(connectionID string) (string, error) {
	if !connectionIDPattern.MatchString(connectionID) {
		return "", ErrInvalidConnectionID
	}
	return filepath.Join(s.root, "connections", connectionID, "output.log"), nil
}
