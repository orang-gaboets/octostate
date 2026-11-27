package plan

import (
	"bytes"
	"context"
	"strings"
	"testing"

	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
)

func TestBuildVerboseLogsPlanCounts(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)
	report, err := Build(ctx, Options{Desired: paddedDesired(), Actual: liveState()})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	want := "plan built for orang-gaboets: actions=0 executable=0 unresolved_memberships=0"
	if len(report.Actions) != 0 || !strings.Contains(logs.String(), want) {
		t.Fatalf("verbose output missing %q (actions=%d):\n%s", want, len(report.Actions), logs.String())
	}
}
