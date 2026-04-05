package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintSuccess(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	data := map[string]string{
		"org":  "acme",
		"repo": "platform",
	}

	if err := PrintSuccess(cmd, "repository created", data); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got %v", err)
	}

	if got["status"] != string(OperationResultStatusSuccess) {
		t.Fatalf("expected status %q, got %#v", OperationResultStatusSuccess, got["status"])
	}
	if got["message"] != "repository created" {
		t.Fatalf("expected message %q, got %#v", "repository created", got["message"])
	}
	if got["data"] == nil {
		t.Fatalf("expected data field, got %#v", got)
	}
}

func TestPrintDryRunOmitsNilData(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := PrintDryRun(cmd, "would delete repository", nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got %v", err)
	}

	if got["status"] != string(OperationResultStatusDryRun) {
		t.Fatalf("expected status %q, got %#v", OperationResultStatusDryRun, got["status"])
	}
	if got["message"] != "would delete repository" {
		t.Fatalf("expected message %q, got %#v", "would delete repository", got["message"])
	}
	if _, exists := got["data"]; exists {
		t.Fatalf("expected omitted data field, got %#v", got["data"])
	}
}
