package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
)

func TestExecuteReturnsBoundedEvidenceAndFullDigest(t *testing.T) {
	t.Parallel()
	for _, tailSignal := range []bool{false, true} {
		for _, exit := range []int{0, 7} {
			t.Run(fmt.Sprintf("tail=%t/exit=%d", tailSignal, exit), func(t *testing.T) {
				t.Parallel()
				raw := "Error: early\n" + strings.Repeat("x", rawevidence.WindowBytes+1)
				wantText := ""
				if tailSignal {
					wantText = "Error: tail 한글\r\n"
					raw = strings.Repeat("x", rawevidence.WindowBytes+1) + "\n" + wantText
				}
				repo := t.TempDir()
				if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte(raw), 0600); err != nil {
					t.Fatal(err)
				}
				var persisted bytes.Buffer
				got, err := ExecuteContextOnly(context.Background(), repo, "unit", []string{"unit"}, "generic", []string{"sh", "-c", fmt.Sprintf("cat input.log; exit %d", exit)}, 30, &persisted)
				if err != nil {
					t.Fatal(err)
				}
				wantStatus := model.RunStatusPassed
				if exit != 0 {
					wantStatus = model.RunStatusFailed
				}
				if got.Status != wantStatus || got.Metadata.ExitCode != exit {
					t.Fatalf("authoritative command outcome: %+v", got)
				}
				snapshot := got.Evidence
				if snapshot.Text != wantText || snapshot.ByteOrigin != int64(len(raw)-len(wantText)) || snapshot.LineOffset != 1 || !snapshot.Oversized || snapshot.TotalBytes != int64(len(raw)) {
					t.Fatalf("unexpected evidence: %+v", snapshot)
				}
				if snapshot.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) || persisted.String() != raw {
					t.Fatal("full raw integrity changed")
				}
			})
		}
	}
}
