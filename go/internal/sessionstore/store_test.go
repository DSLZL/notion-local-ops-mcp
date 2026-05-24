package sessionstore

import "testing"

func TestFSStoreCreatesAndReadsSession(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	session, err := store.Create(SessionInput{
		Shell: "bash",
		CWD:   "/tmp",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if session.ID == "" {
		t.Fatal("session id must not be empty")
	}
	if session.Status != "running" {
		t.Fatalf("Status = %q, want running", session.Status)
	}

	loaded, err := store.Get(session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Shell != "bash" || loaded.CWD != "/tmp" {
		t.Fatalf("loaded session = %+v", loaded)
	}
}

func TestFSStoreAppendsAndReadsOutput(t *testing.T) {
	root := t.TempDir()
	store := NewFSStore(root)

	session, err := store.Create(SessionInput{Shell: "bash"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	nextOffset, err := store.AppendOutput(session.ID, "hello\n")
	if err != nil {
		t.Fatalf("AppendOutput() error = %v", err)
	}

	read, err := store.ReadOutput(session.ID, 0, 1024)
	if err != nil {
		t.Fatalf("ReadOutput() error = %v", err)
	}
	if read.Content != "hello\n" {
		t.Fatalf("Content = %q, want hello\\n", read.Content)
	}
	if read.NextOffset != nextOffset {
		t.Fatalf("NextOffset = %d, want %d", read.NextOffset, nextOffset)
	}
	if read.Truncated {
		t.Fatal("Truncated = true, want false")
	}
}
