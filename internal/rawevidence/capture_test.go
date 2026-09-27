package rawevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestCaptureAcceptedPrefix(t *testing.T) {
	t.Parallel()
	fault := errors.New("raw write failed")
	for _, tc := range []struct {
		name string
		n    int
		err  error
		want error
	}{
		{"full", 6, nil, nil},
		{"short", 3, nil, io.ErrShortWrite},
		{"prefix-and-error", 3, fault, fault},
		{"full-and-error", 6, fault, fault},
		{"zero-and-error", 0, fault, fault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw bytes.Buffer
			calls := 0
			capture := New(writerFunc(func(p []byte) (int, error) {
				calls++
				if calls == 1 {
					return raw.Write(p)
				}
				_, _ = raw.Write(p[:tc.n])
				return tc.n, tc.err
			}))
			if _, err := capture.Write([]byte("start\n")); err != nil {
				t.Fatal(err)
			}
			n, err := capture.Write([]byte("a\nb\nc\n"))
			if n != tc.n || !errors.Is(err, tc.want) || calls != 2 {
				t.Fatalf("write = %d, %v, calls %d", n, err, calls)
			}
			snapshot, err := capture.Snapshot()
			if !errors.Is(err, tc.want) {
				t.Fatalf("capture error = %v, want %v", err, tc.want)
			}
			assertIntegrity(t, snapshot, raw.Bytes())
			if snapshot.Text != raw.String() || snapshot.ByteOrigin != 0 || snapshot.LineOffset != 0 || snapshot.Oversized {
				t.Fatalf("unexpected snapshot: %+v", snapshot)
			}
		})
	}
}

func TestCaptureKeepsFirstError(t *testing.T) {
	t.Parallel()
	first := errors.New("first failure")
	second := errors.New("second failure")
	calls := 0
	capture := New(writerFunc(func(p []byte) (int, error) {
		calls++
		if calls == 1 {
			return 1, first
		}
		return len(p), second
	}))
	_, _ = capture.Write([]byte("ab"))
	_, _ = capture.Write([]byte("cd"))
	snapshot, err := capture.Snapshot()
	if !errors.Is(err, first) || snapshot.Text != "acd" {
		t.Fatalf("snapshot text %q, error %v", snapshot.Text, err)
	}
	assertIntegrity(t, snapshot, []byte("acd"))
}

func TestCaptureBoundedRetentionAndSnapshotOwnership(t *testing.T) {
	t.Parallel()
	capture := New(io.Discard)
	chunk := bytes.Repeat([]byte("line\r\n"), 4096)
	digest := sha256.New()
	var saved Snapshot
	for i := 0; i < 400; i++ {
		if _, err := capture.Write(chunk); err != nil {
			t.Fatal(err)
		}
		_, _ = digest.Write(chunk)
		if capture.retained > WindowBytes+1 || cap(capture.ring[:]) != WindowBytes+1 {
			t.Fatal("capture retention grew")
		}
		if i == 0 {
			saved, _ = capture.Snapshot()
		}
	}
	snapshot, err := capture.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SHA256 != hex.EncodeToString(digest.Sum(nil)) || snapshot.TotalBytes != int64(400*len(chunk)) {
		t.Fatal("full-stream integrity mismatch")
	}
	if len(snapshot.Text) > WindowBytes || !snapshot.Oversized || !strings.HasPrefix(snapshot.Text, "line\r\n") {
		t.Fatal("invalid retained complete-line window")
	}
	if snapshot.ByteOrigin != snapshot.LineOffset*6 {
		t.Fatal("line origin differs from six-byte line boundary")
	}
	clear(chunk)
	if !strings.HasPrefix(saved.Text, "line\r\n") || len(saved.Text) != len(chunk) {
		t.Fatal("snapshot aliases a caller buffer or the mutable ring")
	}
	if _, err := capture.Write([]byte("changed\n")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(snapshot.Text, "changed") {
		t.Fatal("old snapshot changed after another write")
	}
}

func TestCaptureSerializesConcurrentWriters(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer // Intentionally not safe without Capture's serialization.
	capture := New(&raw)
	var writers sync.WaitGroup
	for _, payload := range []string{"first\n", "second\r\n", "한글\n"} {
		writers.Go(func() {
			for i := 0; i < 100; i++ {
				if _, err := capture.Write([]byte(payload)); err != nil {
					t.Error(err)
				}
			}
		})
	}
	writers.Wait()
	snapshot, err := capture.Snapshot()
	if err != nil || snapshot.Text != raw.String() {
		t.Fatalf("serialized evidence mismatch: %v", err)
	}
	assertIntegrity(t, snapshot, raw.Bytes())
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func assertIntegrity(t *testing.T, snapshot Snapshot, raw []byte) {
	t.Helper()
	want := sha256.Sum256(raw)
	if snapshot.TotalBytes != int64(len(raw)) || snapshot.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("integrity mismatch: bytes %d, digest %s", snapshot.TotalBytes, snapshot.SHA256)
	}
}
