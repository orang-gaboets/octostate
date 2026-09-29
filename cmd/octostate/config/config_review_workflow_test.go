//go:build !windows

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigReviewWorkflowExpectedOrganizationForwarding(t *testing.T) {
	workflowBytes, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "config-review.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			WorkflowCall struct {
				Inputs map[string]struct {
					Default any    `yaml:"default"`
					Type    string `yaml:"type"`
				} `yaml:"inputs"`
			} `yaml:"workflow_call"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
				Run  string            `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(workflowBytes, &workflow); err != nil {
		t.Fatal(err)
	}
	if got := workflow.On.WorkflowCall.Inputs["expected_org"]; got.Default != "" || got.Type != "string" {
		t.Fatalf("expected empty organization default, got %#v", got)
	}
	if got := workflow.On.WorkflowCall.Inputs["bind_expected_org"]; got.Default != false || got.Type != "boolean" {
		t.Fatalf("expected opt-in binding default, got %#v", got)
	}

	steps := workflow.Jobs["config-review"].Steps
	for _, stepName := range []string{"Plan configuration", "Preflight configuration apply"} {
		var run string
		for _, step := range steps {
			if step.Name == stepName {
				if step.Env["EXPECTED_ORG"] != "${{ inputs.expected_org }}" {
					t.Fatalf("%s must receive the caller input through its environment", stepName)
				}
				if step.Env["BIND_EXPECTED_ORG"] != "${{ inputs.bind_expected_org }}" {
					t.Fatalf("%s must receive the binding switch through its environment", stepName)
				}
				run = step.Run
				break
			}
		}
		if run == "" {
			t.Fatalf("missing %s workflow step", stepName)
		}

		for _, tc := range []struct {
			name     string
			expected string
			bind     bool
			rejected bool
		}{
			{name: "omitted"},
			{name: "valid", expected: "org-a", bind: true},
			{name: "empty", bind: true},
			{name: "whitespace", expected: "  ", bind: true},
			{name: "former-sentinel", expected: "__OCTOSTATE_EXPECTED_ORG_UNSET__", bind: true},
			{name: "unbound-value", expected: "org-a", rejected: true},
		} {
			t.Run(stepName+"/"+tc.name, func(t *testing.T) {
				command := exec.Command("bash", "-c", "octostate() { printf '%s\\0' \"$@\"; }\n"+run)
				bindValue := "false"
				if tc.bind {
					bindValue = "true"
				}
				command.Env = append(os.Environ(),
					"CONFIG_DIR=./config",
					"OCTOSTATE_TOKEN=token",
					"EXPECTED_ORG="+tc.expected,
					"BIND_EXPECTED_ORG="+bindValue,
				)
				output, err := command.CombinedOutput()
				if tc.rejected {
					if err == nil || !strings.Contains(string(output), "expected_org requires bind_expected_org: true") || strings.Contains(string(output), "\x00") {
						t.Fatalf("expected rejection before octostate, got error %v and output %q", err, output)
					}
					return
				}
				if err != nil {
					t.Fatalf("workflow step failed: %v\n%s", err, output)
				}
				if !strings.HasSuffix(string(output), "\x00") {
					t.Fatalf("workflow step did not invoke octostate: %q", output)
				}
				args := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
				index := slices.Index(args, "--expected-org")
				if !tc.bind && index >= 0 {
					t.Fatalf("unbound call unexpectedly received target flag: %#v", args)
				}
				if tc.bind && (index < 0 || index+1 >= len(args) || args[index+1] != tc.expected) {
					t.Fatalf("caller target %q was not forwarded: %#v", tc.expected, args)
				}
			})
		}
	}
}
