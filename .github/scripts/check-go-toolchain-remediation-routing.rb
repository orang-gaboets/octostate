#!/usr/bin/env ruby

require "yaml"

workflow_path = ".github/workflows/go-toolchain-remediation.yml"
workflow = YAML.safe_load(File.read(workflow_path))
triggers = workflow["on"] || workflow[true]
dispatch_input = triggers.fetch("workflow_dispatch").fetch("inputs").fetch("verify_git_auth")
jobs = workflow.fetch("jobs")
remediate = jobs.fetch("remediate")
verify = jobs.fetch("verify-git-auth")

def require_check(condition, message)
  abort "FAIL: #{message}" unless condition
end

require_check(triggers.keys.sort == %w[schedule workflow_dispatch], "workflow triggers changed")
require_check(dispatch_input["type"] == "boolean", "verification input must be Boolean")
require_check(dispatch_input["required"] == false && dispatch_input["default"] == false, "verification input must default to false")

remediate_condition = remediate.fetch("if")
verify_condition = verify.fetch("if")
require_check(remediate_condition.include?("github.ref == 'refs/heads/main'"), "remediation must remain restricted to main")
require_check(remediate_condition.include?("github.event_name != 'workflow_dispatch' || inputs.verify_git_auth != true"), "verification dispatch must skip remediation while normal dispatch remains enabled")
require_check(verify_condition.include?("github.event_name == 'workflow_dispatch'"), "verification must be manual only")
require_check(verify_condition.include?("inputs.verify_git_auth == true"), "verification must be explicitly enabled")
require_check(verify_condition.include?("github.repository == 'orang-gaboets/octostate'"), "verification must be restricted to this repository")
require_check(verify_condition.include?("github.ref == 'refs/heads/main'"), "verification must be restricted to main")

remediate_group = remediate.fetch("concurrency").fetch("group")
verify_group = verify.fetch("concurrency").fetch("group")
require_check(remediate_group != verify_group, "verification and remediation must use separate concurrency groups")
require_check(verify.fetch("permissions") == { "contents" => "read" }, "verification job must have read-only repository permissions")
require_check(verify.fetch("environment").fetch("name") == remediate.fetch("environment").fetch("name"), "verification must use the protected remediation environment")

steps = verify.fetch("steps")
require_check(steps.length == 4, "verification job must contain only checkout, SHA check, token, and read-only probe steps")
checkout = steps.fetch(0)
require_check(checkout.fetch("uses").start_with?("actions/checkout@"), "verification must checkout the dispatched source")
require_check(checkout.fetch("with").fetch("ref") == "${{ github.sha }}", "verification must checkout the dispatched SHA")
require_check(checkout.fetch("with").fetch("persist-credentials") == false, "checkout credentials must not persist")

token_step = steps.find { |step| step["name"] == "Generate repository-scoped read-only GitHub App token" }
require_check(token_step, "read-only App token step is missing")
token_inputs = token_step.fetch("with")
require_check(token_inputs["owner"] == "orang-gaboets" && token_inputs["repositories"] == "octostate", "App token must be scoped to this repository")
require_check(token_inputs["permission-contents"] == "read", "App token must have contents:read")

probe_step = steps.find { |step| step["name"] == "Compare GitHub App authentication schemes" }
require_check(probe_step, "authentication comparison step is missing")
probe_script = probe_step.fetch("run")
%w[invalid-basic-main bearer-main basic-main basic-absent].each do |label|
  require_check(probe_script.include?(label), "probe result #{label} must be recorded")
end
require_check(probe_script.include?("git_remote_branch_result"), "probe must use the production authentication helper")
require_check(probe_script.include?("GITHUB_RUN_ID") && probe_script.include?("GITHUB_RUN_ATTEMPT"), "absent-ref probe must use a run-specific branch name")
%w[auth-failed:128 present:0 absent:2].each do |result|
  require_check(probe_script.include?(result), "probe must assert the expected result and Git exit status #{result}")
end
require_check(!probe_script.match?(/\b(?:gh\s+(?:pr|api)|git\s+(?:push|update-ref)|classify|go-toolchain-remediation\s+(?:apply|existing))\b/), "verification must not run mutation or remediation commands")

puts "go toolchain remediation workflow routing checks passed"
