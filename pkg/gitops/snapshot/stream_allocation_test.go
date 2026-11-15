//go:build !race

package snapshot

import (
	"io"
	"runtime"
	"strings"
	"testing"
)

func TestWriteActualSnapshotJSONBoundsSortingForOverLimitMap(t *testing.T) {
	snapshot := ActualSnapshot{ResolvedInviteUserIDsByUsername: overLimitResolvedInviteIDs()}
	runtime.GC()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := writeActualSnapshotJSON(io.Discard, snapshot, actualSnapshotJSONLimits(), maxActualSnapshotJSONSize)
	runtime.ReadMemStats(&after)

	if err == nil || !strings.Contains(err.Error(), "JSON object entry limit") {
		t.Fatalf("writeActualSnapshotJSON error = %v, want object entry limit", err)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 27<<20 {
		t.Fatalf("writeActualSnapshotJSON allocated %d bytes for an over-limit map, want under 27 MiB", allocated)
	}
}
