package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestCaptureMatchesBoundedTail(t *testing.T) {
	t.Parallel()
	if rawevidence.WindowBytes != safety.MaxRegexInputBytes {
		t.Fatal("capture and extraction limits differ")
	}
	w := rawevidence.WindowBytes
	inputs := []string{
		"", "one", "one\n", "\r\n한글\x1b[31mFAIL\x1b[0m\nlast",
		strings.Repeat("x", w), strings.Repeat("x", w+1),
		"\n" + strings.Repeat("x", w),
		strings.Repeat("x", w) + "\n",
		strings.Repeat("x", w) + "\nlast",
		strings.Repeat("line\r\n", w/3) + "\x1b[31mFAIL 한글\x1b[0m",
	}
	for i, raw := range inputs {
		wantText, wantByte, wantLine, wantOversized := boundedTail(raw)
		wantHash := sha256.Sum256([]byte(raw))
		for _, chunk := range []int{1, 7, 4093, w, len(raw) + 1} {
			t.Run(fmt.Sprintf("input-%d/chunk-%d", i, chunk), func(t *testing.T) {
				capture := rawevidence.New(io.Discard)
				for off := 0; off < len(raw); off += chunk {
					if _, err := capture.Write([]byte(raw[off:min(off+chunk, len(raw))])); err != nil {
						t.Fatal(err)
					}
				}
				got, err := capture.Snapshot()
				if err != nil {
					t.Fatal(err)
				}
				if got.Text != wantText || got.ByteOrigin != int64(wantByte) || got.LineOffset != int64(wantLine) || got.Oversized != wantOversized {
					t.Fatalf("tail mismatch: bytes %d/%d, origin %d/%d, line offset %d/%d, oversized %v/%v", len(got.Text), len(wantText), got.ByteOrigin, wantByte, got.LineOffset, wantLine, got.Oversized, wantOversized)
				}
				if got.TotalBytes != int64(len(raw)) || got.SHA256 != hex.EncodeToString(wantHash[:]) {
					t.Fatal("accepted-stream count or digest mismatch")
				}
			})
		}
	}
}
