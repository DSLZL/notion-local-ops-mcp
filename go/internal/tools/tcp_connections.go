package tools

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"notion-local-ops-mcp-go/internal/connectionstore"
)

var errTCPConnectionNotActive = errors.New("tcp connection is inactive or lost")

type TCPConnectOptions struct {
	Host           string
	Port           int
	TimeoutSeconds int
}

type TCPSendOptions struct {
	Text          string
	ContentBase64 string
	AppendNewline bool
}

type TCPReadOptions struct {
	TimeoutSeconds  int
	MaxBytes        int
	ReadUntil       string
	ReadUntilBase64 string
	OutputMode      string
}

type TCPConnectionResult struct {
	Success    bool                       `json:"success"`
	Connection connectionstore.Connection `json:"connection"`
	Active     bool                       `json:"active"`
}

type TCPSendResult struct {
	Success      bool   `json:"success"`
	ConnectionID string `json:"connection_id"`
	BytesWritten int    `json:"bytes_written"`
	Summary      string `json:"summary"`
}

type TCPReadResult struct {
	Success          bool   `json:"success"`
	ConnectionID     string `json:"connection_id"`
	Content          string `json:"content,omitempty"`
	ContentBase64    string `json:"content_base64,omitempty"`
	OutputMode       string `json:"output_mode"`
	BytesRead        int    `json:"bytes_read"`
	DelimiterMatched bool   `json:"delimiter_matched"`
	TimedOut         bool   `json:"timed_out"`
	Truncated        bool   `json:"truncated"`
}

type activeTCPConnection struct {
	conn net.Conn
}

var (
	activeTCPConnectionsMu sync.RWMutex
	activeTCPConnections   = map[string]*activeTCPConnection{}
)

func TCPConnect(stateDir string, options TCPConnectOptions) (TCPConnectionResult, error) {
	if options.Host == "" {
		return TCPConnectionResult{}, errors.New("host is required")
	}
	if options.Port <= 0 || options.Port > 65535 {
		return TCPConnectionResult{}, errors.New("port must be between 1 and 65535")
	}

	timeout := tcpTimeout(options.TimeoutSeconds, 5*time.Second)
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", net.JoinHostPort(options.Host, strconv.Itoa(options.Port)))
	if err != nil {
		return TCPConnectionResult{}, err
	}

	store := connectionstore.NewFSStore(stateDir)
	persisted, err := store.Create(connectionstore.ConnectionInput{
		Network:    "tcp",
		Host:       options.Host,
		Port:       options.Port,
		RemoteAddr: conn.RemoteAddr().String(),
	})
	if err != nil {
		_ = conn.Close()
		return TCPConnectionResult{}, err
	}

	registerActiveTCPConnection(persisted.ID, &activeTCPConnection{conn: conn})
	return TCPConnectionResult{
		Success:    true,
		Connection: persisted,
		Active:     true,
	}, nil
}

func TCPSend(stateDir, connectionID string, options TCPSendOptions) (TCPSendResult, error) {
	active := getActiveTCPConnection(connectionID)
	if active == nil {
		markTCPConnectionLostIfPersisted(stateDir, connectionID)
		return TCPSendResult{}, errTCPConnectionNotActive
	}

	hasText := options.Text != ""
	hasBase64 := options.ContentBase64 != ""
	if hasText == hasBase64 {
		return TCPSendResult{}, errors.New("exactly one of text or content_base64 must be set")
	}
	if hasBase64 && options.AppendNewline {
		return TCPSendResult{}, errors.New("append_newline only supports text mode")
	}

	var payload []byte
	if hasText {
		text := options.Text
		if options.AppendNewline {
			text += "\n"
		}
		payload = []byte(text)
	} else {
		decoded, err := base64.StdEncoding.DecodeString(options.ContentBase64)
		if err != nil {
			return TCPSendResult{}, fmt.Errorf("invalid content_base64: %w", err)
		}
		payload = decoded
	}

	n, err := active.conn.Write(payload)
	if err != nil {
		return TCPSendResult{}, err
	}

	store := connectionstore.NewFSStore(stateDir)
	if _, err := store.AppendInput(connectionID, string(payload[:n])); err != nil {
		return TCPSendResult{}, err
	}

	return TCPSendResult{
		Success:      true,
		ConnectionID: connectionID,
		BytesWritten: n,
		Summary:      "payload delivered",
	}, nil
}

func TCPRead(stateDir, connectionID string, options TCPReadOptions) (TCPReadResult, error) {
	active := getActiveTCPConnection(connectionID)
	if active == nil {
		markTCPConnectionLostIfPersisted(stateDir, connectionID)
		return TCPReadResult{}, errTCPConnectionNotActive
	}

	if options.ReadUntil != "" && options.ReadUntilBase64 != "" {
		return TCPReadResult{}, errors.New("read_until and read_until_base64 cannot both be set")
	}

	outputMode := options.OutputMode
	if outputMode == "" {
		outputMode = "text"
	}
	if outputMode != "text" && outputMode != "base64" {
		return TCPReadResult{}, errors.New("output_mode must be text or base64")
	}

	var delimiter []byte
	if options.ReadUntil != "" {
		delimiter = []byte(options.ReadUntil)
	} else if options.ReadUntilBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(options.ReadUntilBase64)
		if err != nil {
			return TCPReadResult{}, fmt.Errorf("invalid read_until_base64: %w", err)
		}
		delimiter = decoded
	}

	timeout := tcpTimeout(options.TimeoutSeconds, 5*time.Second)
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 4096
	}

	buf := make([]byte, 0, maxBytes)
	chunkSize := 1024
	if chunkSize > maxBytes {
		chunkSize = maxBytes
	}
	chunk := make([]byte, chunkSize)
	delimiterMatched := false
	timedOut := false

	for len(buf) < maxBytes {
		remaining := maxBytes - len(buf)
		readBuf := chunk
		if remaining < len(readBuf) {
			readBuf = readBuf[:remaining]
		}

		_ = active.conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := active.conn.Read(readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
			if len(delimiter) > 0 && bytes.Contains(buf, delimiter) {
				delimiterMatched = true
				break
			}
			if len(delimiter) == 0 {
				break
			}
			continue
		}

		if err == nil {
			break
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			timedOut = true
			break
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return TCPReadResult{}, err
	}
	_ = active.conn.SetReadDeadline(time.Time{})

	store := connectionstore.NewFSStore(stateDir)
	if len(buf) > 0 {
		if _, err := store.AppendOutput(connectionID, string(buf)); err != nil {
			return TCPReadResult{}, err
		}
	}

	result := TCPReadResult{
		Success:          true,
		ConnectionID:     connectionID,
		OutputMode:       outputMode,
		BytesRead:        len(buf),
		DelimiterMatched: delimiterMatched,
		TimedOut:         timedOut,
		Truncated:        len(buf) >= maxBytes && !delimiterMatched,
	}
	if outputMode == "base64" {
		result.ContentBase64 = base64.StdEncoding.EncodeToString(buf)
	} else {
		result.Content = string(buf)
	}
	return result, nil
}

func TCPClose(stateDir, connectionID string) (TCPConnectionResult, error) {
	active := unregisterActiveTCPConnection(connectionID)
	store := connectionstore.NewFSStore(stateDir)

	connection, err := store.Get(connectionID)
	if err != nil {
		if active != nil {
			_ = active.conn.Close()
		}
		return TCPConnectionResult{}, errTCPConnectionNotActive
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	connection.UpdatedAt = now
	connection.FinishedAt = now

	if active != nil {
		_ = active.conn.Close()
		connection.Status = "closed"
	} else {
		connection.Status = "lost"
	}

	updated, err := store.Update(connectionID, connection)
	if err != nil {
		return TCPConnectionResult{}, err
	}
	return TCPConnectionResult{
		Success:    true,
		Connection: updated,
		Active:     false,
	}, nil
}

func registerActiveTCPConnection(connectionID string, active *activeTCPConnection) {
	activeTCPConnectionsMu.Lock()
	defer activeTCPConnectionsMu.Unlock()
	activeTCPConnections[connectionID] = active
}

func getActiveTCPConnection(connectionID string) *activeTCPConnection {
	activeTCPConnectionsMu.RLock()
	defer activeTCPConnectionsMu.RUnlock()
	return activeTCPConnections[connectionID]
}

func unregisterActiveTCPConnection(connectionID string) *activeTCPConnection {
	activeTCPConnectionsMu.Lock()
	defer activeTCPConnectionsMu.Unlock()
	active := activeTCPConnections[connectionID]
	delete(activeTCPConnections, connectionID)
	return active
}

func markTCPConnectionLostIfPersisted(stateDir, connectionID string) {
	store := connectionstore.NewFSStore(stateDir)
	connection, err := store.Get(connectionID)
	if err != nil {
		return
	}
	if connection.Status == "closed" || connection.Status == "lost" {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	connection.Status = "lost"
	connection.UpdatedAt = now
	if connection.FinishedAt == "" {
		connection.FinishedAt = now
	}
	if connection.LastError == "" {
		connection.LastError = errTCPConnectionNotActive.Error()
	}
	_, _ = store.Update(connectionID, connection)
}

func tcpTimeout(timeoutSeconds int, fallback time.Duration) time.Duration {
	if timeoutSeconds <= 0 {
		return fallback
	}
	return time.Duration(timeoutSeconds) * time.Second
}
