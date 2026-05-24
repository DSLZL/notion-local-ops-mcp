package tools

import (
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"notion-local-ops-mcp-go/internal/connectionstore"
)

type tcpTestServer struct {
	Host     string
	Port     int
	listener net.Listener
}

func startTCPTestServer(t *testing.T, handler func(conn net.Conn)) *tcpTestServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	addr := ln.Addr().String()
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("net.SplitHostPort() error = %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("strconv.Atoi() error = %v", err)
	}

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				handler(c)
			}(conn)
		}
	}()

	return &tcpTestServer{
		Host:     host,
		Port:     port,
		listener: ln,
	}
}

func TestTCPConnectionLifecycleSupportsPromptStyleFlow(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("name: "))
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		_, _ = conn.Write([]byte("hello " + strings.TrimSpace(string(buf[:n])) + "\n> "))
	})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	first, err := TCPRead(stateDir, connected.Connection.ID, TCPReadOptions{
		TimeoutSeconds: 2,
		ReadUntil:      "name: ",
		MaxBytes:       1024,
	})
	if err != nil {
		t.Fatalf("TCPRead(first) error = %v", err)
	}
	if !strings.Contains(first.Content, "name: ") {
		t.Fatalf("first content = %q, want prompt", first.Content)
	}

	if _, err := TCPSend(stateDir, connected.Connection.ID, TCPSendOptions{
		Text:          "alice",
		AppendNewline: true,
	}); err != nil {
		t.Fatalf("TCPSend() error = %v", err)
	}

	second, err := TCPRead(stateDir, connected.Connection.ID, TCPReadOptions{
		TimeoutSeconds: 2,
		ReadUntil:      "> ",
		MaxBytes:       1024,
	})
	if err != nil {
		t.Fatalf("TCPRead(second) error = %v", err)
	}
	if !strings.Contains(second.Content, "hello alice") {
		t.Fatalf("second content = %q, want greeting", second.Content)
	}
}

func TestTCPSendSupportsBase64Payloads(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {
		buf := make([]byte, 4)
		n, _ := io.ReadFull(conn, buf)
		_, _ = conn.Write(buf[:n])
	})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	if _, err := TCPSend(stateDir, connected.Connection.ID, TCPSendOptions{
		ContentBase64: "AAECAw==",
	}); err != nil {
		t.Fatalf("TCPSend(base64) error = %v", err)
	}

	read, err := TCPRead(stateDir, connected.Connection.ID, TCPReadOptions{
		TimeoutSeconds: 2,
		MaxBytes:       4,
		OutputMode:     "base64",
	})
	if err != nil {
		t.Fatalf("TCPRead(base64) error = %v", err)
	}
	if read.ContentBase64 != "AAECAw==" {
		t.Fatalf("ContentBase64 = %q, want AAECAw==", read.ContentBase64)
	}
}

func TestTCPReadTimeoutAndCloseLifecycle(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {
		time.Sleep(1500 * time.Millisecond)
		_, _ = conn.Write([]byte("late\n"))
	})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	read, err := TCPRead(stateDir, connected.Connection.ID, TCPReadOptions{
		TimeoutSeconds: 1,
		MaxBytes:       1024,
	})
	if err != nil {
		t.Fatalf("TCPRead(timeout) error = %v", err)
	}
	if !read.TimedOut {
		t.Fatal("TimedOut = false, want true")
	}

	closed, err := TCPClose(stateDir, connected.Connection.ID)
	if err != nil {
		t.Fatalf("TCPClose() error = %v", err)
	}
	if closed.Active {
		t.Fatal("Active = true, want false")
	}
	if closed.Connection.Status != "closed" {
		t.Fatalf("Status = %q, want closed", closed.Connection.Status)
	}
}

func TestTCPReadMarksPersistedConnectionLostWhenLiveSocketIsGone(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {
		time.Sleep(2 * time.Second)
		_, _ = conn.Write([]byte("unused"))
	})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	active := unregisterActiveTCPConnection(connected.Connection.ID)
	if active == nil {
		t.Fatal("expected active TCP connection to exist")
	}
	_ = active.conn.Close()

	if _, err := TCPRead(stateDir, connected.Connection.ID, TCPReadOptions{TimeoutSeconds: 1}); err == nil {
		t.Fatal("TCPRead() error = nil, want inactive/lost error")
	}

	store := connectionstore.NewFSStore(stateDir)
	connection, err := store.Get(connected.Connection.ID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if connection.Status != "lost" {
		t.Fatalf("Status = %q, want lost", connection.Status)
	}
	if connection.LastError == "" {
		t.Fatal("LastError must be recorded for lost connection")
	}
	if connection.FinishedAt == "" {
		t.Fatal("FinishedAt must be set for lost connection")
	}
}

func TestTCPSendMarksPersistedConnectionLostWhenLiveSocketIsGone(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {
		time.Sleep(2 * time.Second)
	})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	active := unregisterActiveTCPConnection(connected.Connection.ID)
	if active == nil {
		t.Fatal("expected active TCP connection to exist")
	}
	_ = active.conn.Close()

	if _, err := TCPSend(stateDir, connected.Connection.ID, TCPSendOptions{Text: "hello"}); err == nil {
		t.Fatal("TCPSend() error = nil, want inactive/lost error")
	}

	store := connectionstore.NewFSStore(stateDir)
	connection, err := store.Get(connected.Connection.ID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if connection.Status != "lost" {
		t.Fatalf("Status = %q, want lost", connection.Status)
	}
}

func TestTCPCloseMarksPersistedConnectionLostWithoutLiveSocket(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {
		time.Sleep(2 * time.Second)
	})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	active := unregisterActiveTCPConnection(connected.Connection.ID)
	if active == nil {
		t.Fatal("expected active TCP connection to exist")
	}
	_ = active.conn.Close()

	closed, err := TCPClose(stateDir, connected.Connection.ID)
	if err != nil {
		t.Fatalf("TCPClose() error = %v", err)
	}
	if closed.Active {
		t.Fatal("Active = true, want false")
	}
	if closed.Connection.Status != "lost" {
		t.Fatalf("Status = %q, want lost", closed.Connection.Status)
	}
}

func TestTCPSendRejectsInvalidPayloadShapes(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	tests := []struct {
		name    string
		options TCPSendOptions
		wantErr string
	}{
		{
			name:    "missing payload",
			options: TCPSendOptions{},
			wantErr: "exactly one of text or content_base64 must be set",
		},
		{
			name: "conflicting payloads",
			options: TCPSendOptions{
				Text:          "hello",
				ContentBase64: "aGVsbG8=",
			},
			wantErr: "exactly one of text or content_base64 must be set",
		},
		{
			name: "append newline in base64 mode",
			options: TCPSendOptions{
				ContentBase64: "aGVsbG8=",
				AppendNewline: true,
			},
			wantErr: "append_newline only supports text mode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := TCPSend(stateDir, connected.Connection.ID, tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("TCPSend() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestTCPReadRejectsInvalidDelimiterAndOutputOptions(t *testing.T) {
	stateDir := t.TempDir()
	server := startTCPTestServer(t, func(conn net.Conn) {})
	_ = server

	connected, err := TCPConnect(stateDir, TCPConnectOptions{
		Host:           server.Host,
		Port:           server.Port,
		TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("TCPConnect() error = %v", err)
	}

	tests := []struct {
		name    string
		options TCPReadOptions
		wantErr string
	}{
		{
			name: "conflicting delimiters",
			options: TCPReadOptions{
				ReadUntil:       "> ",
				ReadUntilBase64: "PiA=",
			},
			wantErr: "read_until and read_until_base64 cannot both be set",
		},
		{
			name: "invalid output mode",
			options: TCPReadOptions{
				OutputMode: "hex",
			},
			wantErr: "output_mode must be text or base64",
		},
		{
			name: "invalid base64 delimiter",
			options: TCPReadOptions{
				ReadUntilBase64: "%%%invalid%%%",
			},
			wantErr: "invalid read_until_base64",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := TCPRead(stateDir, connected.Connection.ID, tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("TCPRead() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
