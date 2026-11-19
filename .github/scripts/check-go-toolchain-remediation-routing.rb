#!/usr/bin/env ruby

require "yaml"

workflow_path = ".github/workflows/go-toolchain-remediation.yml"
workflow = YAML.safe_load(File.read(workflow_path))
triggers = workflow["on"] || workflow[true]
jobs = workflow.fetch("jobs")
remediate = jobs.fetch("remediate")

def require_check(condition, message)
  abort "FAIL: #{message}" unless condition
end

require_check(triggers.keys.sort == %w[schedule workflow_dispatch], "workflow triggers changed")
require_check(triggers.fetch("workflow_dispatch") == {}, "manual dispatch must not expose temporary authentication inputs")
require_check(jobs.keys == ["remediate"], "only the normal remediation job should remain")
require_check(
  remediate.fetch("if") == "github.ref == 'refs/heads/main'",
  "remediation must remain restricted to main for scheduled and manual runs"
)
require_check(workflow.fetch("permissions") == { "contents" => "read" }, "workflow must retain read-only default permissions")

concurrency = remediate.fetch("concurrency")
require_check(concurrency.fetch("group") == 'go-toolchain-remediation-${{ github.repository }}', "remediation runs must serialize per repository")
require_check(concurrency.fetch("cancel-in-progress") == false, "running remediation jobs must not be cancelled")

environment = remediate.fetch("environment")
require_check(environment.fetch("name") == "go-toolchain-remediation", "remediation must retain its protected environment")
require_check(environment.fetch("deployment") == false, "remediation environment must not create deployment records")

puts "go toolchain remediation workflow routing checks passed"
