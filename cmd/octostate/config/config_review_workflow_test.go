package config

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigReviewWorkflowExpectedOrganizationForwarding(t *testing.T) {
	const sentinel = "__OCTOSTATE_EXPECTED_ORG_UNSET__"
	workflowBytes, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "config-review.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			WorkflowCall struct {
				Inputs map[string]struct {
					Default string `yaml:"default"`
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
	if got := workflow.On.WorkflowCall.Inputs["expected_org"].Default; got != sentinel {
		t.Fatalf("expected reserved workflow default %q, got %q", sentinel, got)
	}

	steps := workflow.Jobs["config-review"].Steps
	for _, stepName := range []string{"Plan configuration", "Preflight configuration apply"} {
		var run string
		for _, step := range steps {
			if step.Name == stepName {
				if step.Env["EXPECTED_ORG"] != "${{ inputs.expected_org }}" {
					t.Fatalf("%s must receive the caller input through its environment", stepName)
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
			bound    bool
		}{
			{name: "omitted", expected: sentinel},
			{name: "valid", expected: "org-a", bound: true},
			{name: "empty", expected: "", bound: true},
			{name: "whitespace", expected: "  ", bound: true},
		} {
			t.Run(stepName+"/"+tc.name, func(t *testing.T) {
				tmp := t.TempDir()
				stub := filepath.Join(tmp, "octostate")
				if err := os.WriteFile(stub, []byte("#!/usr/bin/env bash\nprintf '%s\\0' \"$@\" >\"$ARG_LOG\"\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				log := filepath.Join(tmp, "args")
				command := exec.Command("bash", "-c", run)
				command.Env = append(os.Environ(),
					"PATH="+tmp+":"+os.Getenv("PATH"),
					"ARG_LOG="+log,
					"CONFIG_DIR=./config",
					"OCTOSTATE_TOKEN=token",
					"EXPECTED_ORG="+tc.expected,
				)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("workflow step failed: %v\n%s", err, output)
				}
				data, err := os.ReadFile(log)
				if err != nil {
					t.Fatalf("workflow step did not invoke octostate: %v", err)
				}
				parts := bytes.Split(data, []byte{0})
				args := make([]string, 0, len(parts)-1)
				for _, part := range parts[:len(parts)-1] {
					args = append(args, string(part))
				}
				index := slices.Index(args, "--expected-org")
				if !tc.bound && index >= 0 {
					t.Fatalf("unbound call unexpectedly received target flag: %#v", args)
				}
				if tc.bound && (index < 0 || index+1 >= len(args) || args[index+1] != tc.expected) {
					t.Fatalf("caller target %q was not forwarded: %#v", tc.expected, args)
				}
			})
		}
	}
}
