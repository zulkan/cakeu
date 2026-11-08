package attachment_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goskeleton/internal/attachment"
)

func TestLocalStoreRoundTripsBytes(t *testing.T) {
	store, err := attachment.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("expected store creation to succeed, got error: %v", err)
	}

	payload := []byte("receipt bytes")
	written, err := store.Save("11111111-1111-1111-1111-111111111111.pdf", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}
	if written != int64(len(payload)) {
		t.Fatalf("expected %d bytes written, got %d", len(payload), written)
	}

	reader, err := store.Open("11111111-1111-1111-1111-111111111111.pdf")
	if err != nil {
		t.Fatalf("expected open to succeed, got error: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("expected read to succeed, got error: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, got)
	}
}

func TestLocalStoreReopensSavedFileFromNewStore(t *testing.T) {
	dir := t.TempDir()

	first, err := attachment.NewLocalStore(dir)
	if err != nil {
		t.Fatalf("expected store creation to succeed, got error: %v", err)
	}
	if _, err := first.Save("kept.png", strings.NewReader("png")); err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}

	second, err := attachment.NewLocalStore(dir)
	if err != nil {
		t.Fatalf("expected second store creation to succeed, got error: %v", err)
	}
	reader, err := second.Open("kept.png")
	if err != nil {
		t.Fatalf("expected open after reopen to succeed, got error: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("expected read to succeed, got error: %v", err)
	}
	if string(got) != "png" {
		t.Fatalf("expected %q, got %q", "png", got)
	}
}

func TestLocalStoreRemovesFile(t *testing.T) {
	dir := t.TempDir()
	store, err := attachment.NewLocalStore(dir)
	if err != nil {
		t.Fatalf("expected store creation to succeed, got error: %v", err)
	}
	if _, err := store.Save("gone.pdf", strings.NewReader("pdf")); err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}

	if err := store.Remove("gone.pdf"); err != nil {
		t.Fatalf("expected remove to succeed, got error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "gone.pdf")); !os.IsNotExist(err) {
		t.Fatalf("expected file to be gone, got stat error: %v", err)
	}
}

func TestLocalStoreRejectsPathTraversal(t *testing.T) {
	store, err := attachment.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("expected store creation to succeed, got error: %v", err)
	}

	for _, key := range []string{"../escape.png", "nested/escape.png", `nested\escape.png`, "..", "", "."} {
		t.Run(key, func(t *testing.T) {
			if _, err := store.Save(key, strings.NewReader("x")); err == nil {
				t.Fatalf("expected Save(%q) to be rejected", key)
			}
			if _, err := store.Open(key); err == nil {
				t.Fatalf("expected Open(%q) to be rejected", key)
			}
			if err := store.Remove(key); err == nil {
				t.Fatalf("expected Remove(%q) to be rejected", key)
			}
		})
	}
}
