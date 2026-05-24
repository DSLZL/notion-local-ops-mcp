package connectionstore

import "testing"

func TestFSStoreCreatesAndReadsConnection(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	conn, err := store.Create(ConnectionInput{
		Network: "tcp",
		Host:    "127.0.0.1",
		Port:    31337,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if conn.ID == "" {
		t.Fatal("connection id must not be empty")
	}
	if conn.Status != "open" {
		t.Fatalf("Status = %q, want open", conn.Status)
	}

	loaded, err := store.Get(conn.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Host != "127.0.0.1" || loaded.Port != 31337 {
		t.Fatalf("loaded connection = %+v", loaded)
	}
}

func TestFSStoreAppendsAndReadsLogs(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)
	conn, err := store.Create(ConnectionInput{
		Network: "tcp",
		Host:    "127.0.0.1",
		Port:    9001,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	nextOffset, err := store.AppendOutput(conn.ID, "banner\n")
	if err != nil {
		t.Fatalf("AppendOutput() error = %v", err)
	}
	read, err := store.ReadOutput(conn.ID, 0, 1024)
	if err != nil {
		t.Fatalf("ReadOutput() error = %v", err)
	}
	if read.Content != "banner\n" || read.NextOffset != nextOffset {
		t.Fatalf("read = %+v, nextOffset = %d", read, nextOffset)
	}
}
