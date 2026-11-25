#!/usr/bin/env ruby

require "yaml"

workflow_path = ENV.fetch("ACTIONS_WATCHER_WORKFLOW_PATH", ".github/workflows/actions-watcher.yml")
source_path = ENV.fetch("ACTIONS_WATCHER_SOURCE_PATH", "cmd/actions-watcher/github.go")
workflow = YAML.safe_load(File.read(workflow_path), aliases: true)
source = File.read(source_path)
triggers = workflow["on"] || workflow[true]
jobs = workflow.fetch("jobs")
job = jobs.fetch("reconcile")

EXPECTED_WORKFLOWS = [
  "CI",
  "CodeQL Security",
  "Go vulnerability monitoring",
  "Go toolchain remediation",
  "Release Please",
  "Contributor showcase",
  "Trusted Live Integration"
].freeze

EXPECTED_CHECKOUT = {
  "uses" => "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
  "with" => {
    "ref" => "main",
    "fetch-depth" => 0,
    "persist-credentials" => false
  }
}.freeze

EXPECTED_SETUP_GO = {
  "uses" => "actions/setup-go@d8cbb8045d7f43706b9177cceb36c1f91a6644d5",
  "with" => {
    "go-version-file" => "go.mod",
    "cache" => true
  }
}.freeze

EXPECTED_APP_TOKEN = {
  "uses" => "actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510",
  "with" => {
    "app-id" => "${{ vars.ACTIONS_WATCHER_APP_ID }}",
    "private-key" => "${{ secrets.ACTIONS_WATCHER_PRIVATE_KEY }}",
    "owner" => "orang-gaboets",
    "repositories" => "octostate",
    "permission-actions" => "read",
    "permission-issues" => "write",
    "permission-metadata" => "read"
  }
}.freeze

def require_check(condition, message)
  abort "FAIL: #{message}" unless condition
end

require_check(!workflow.key?("env") && !job.key?("env"), "workflow and reconcile job must not define inherited environment variables")
require_check(triggers.keys.sort == %w[push schedule workflow_run], "only push, schedule, and workflow_run triggers are allowed")
require_check(triggers.fetch("push") == { "branches" => ["main"] }, "push reconciliation must be restricted to main")
require_check(triggers.fetch("schedule") == [{ "cron" => "23 * * * *" }], "hourly reconciliation schedule changed")
workflow_run = triggers.fetch("workflow_run")
require_check(workflow_run["branches"] == ["main"], "workflow_run reconciliation must be restricted to main source branches")
require_check(workflow_run.fetch("types") == ["completed"], "workflow_run must run only for completed source runs")
require_check(workflow_run.fetch("workflows") == EXPECTED_WORKFLOWS, "workflow_run allowlist changed")
require_check(workflow.fetch("permissions") == { "contents" => "read" }, "workflow permissions must remain read-only")
require_check(jobs.keys == ["reconcile"], "only the reconcile job should remain")
require_check(job.fetch("if") == "github.ref == 'refs/heads/main' && (github.event_name != 'workflow_run' || contains(fromJSON('[\"push\",\"schedule\",\"workflow_dispatch\"]'), github.event.workflow_run.event))", "reconciliation must run only from main and eligible workflow_run source events")
require_check(job.fetch("permissions") == { "contents" => "read" }, "job token must retain read-only contents permission")

concurrency = workflow.fetch("concurrency")
require_check(concurrency.fetch("group") == 'actions-watcher-${{ github.repository }}', "watcher runs must serialize per repository")
require_check(concurrency.fetch("cancel-in-progress") == false, "running watcher jobs must not be cancelled")

environment = job.fetch("environment")
require_check(environment.fetch("name") == "actions-watcher", "watcher must use its protected environment")
require_check(environment.fetch("deployment") == false, "watcher environment must not create deployment records")

steps = job.fetch("steps")
expected_step_names = [
  "Checkout trusted main",
  "Set up Go",
  "Derive watcher activation time",
  "Delay push reconciliation",
  "Create scoped watcher installation token",
  "Reconcile workflow incidents"
]
require_check(steps.map { |step| step["name"] } == expected_step_names, "workflow steps must match the reviewed token-safe sequence")
checkout = steps.find { |step| step["name"] == "Checkout trusted main" }
require_check(checkout && checkout["uses"] == EXPECTED_CHECKOUT.fetch("uses"), "checkout action revision changed")
require_check(checkout.fetch("with") == EXPECTED_CHECKOUT.fetch("with"), "checkout inputs changed")

setup_go = steps.find { |step| step["name"] == "Set up Go" }
require_check(setup_go && setup_go["uses"] == EXPECTED_SETUP_GO.fetch("uses"), "setup-go action revision changed")
require_check(setup_go.fetch("with") == EXPECTED_SETUP_GO.fetch("with"), "setup-go inputs changed")

app_token = steps.find { |step| step["name"] == "Create scoped watcher installation token" }
require_check(app_token && app_token["uses"] == EXPECTED_APP_TOKEN.fetch("uses"), "App token action revision changed")
require_check(app_token.fetch("with") == EXPECTED_APP_TOKEN.fetch("with"), "App token inputs changed")

activation = steps.find { |step| step["name"] == "Derive watcher activation time" }
activation_script = <<~'SCRIPT'.strip
  set -euo pipefail
  activated_at="$(git log --first-parent --diff-filter=A --format=%cI -1 -- .github/workflows/actions-watcher.yml)"
  if [[ -z "$activated_at" ]]; then
    echo 'Unable to find the first-parent commit that added the watcher workflow.' >&2
    exit 1
  fi
  echo "activated-at=$activated_at" >> "$GITHUB_OUTPUT"
SCRIPT
require_check(!activation.nil? && activation.fetch("run").strip == activation_script && activation.fetch("shell") == "bash", "activation step must run only the reviewed trusted-main script")
delay = steps.find { |step| step["name"] == "Delay push reconciliation" }
require_check(!delay.nil? && delay.fetch("if") == "github.event_name == 'push'" && delay.fetch("shell") == "bash" && delay.fetch("run") == "sleep 900", "main push reconciliation must run only the reviewed 15-minute delay")
command = steps.find { |step| step["name"] == "Reconcile workflow incidents" }
require_check(!command.nil? && command.fetch("run") == "go run ./cmd/actions-watcher", "workflow must invoke the watcher command")
require_check(steps.reject { |step| step.equal?(command) }.none? { |step| step.key?("env") }, "only the reconciliation command may define step environment variables")
command_env = command.fetch("env")
require_check(command_env == {
  "ACTIONS_WATCHER_ACTIVATED_AT" => "${{ steps.activation.outputs.activated-at }}",
  "ACTIONS_WATCHER_APP_SLUG" => "${{ steps.app-token.outputs.app-slug }}",
  "ACTIONS_WATCHER_TOKEN" => "${{ steps.app-token.outputs.token }}",
  "GITHUB_API_URL" => "${{ github.api_url }}",
  "GITHUB_EVENT_NAME" => "${{ github.event_name }}",
  "GITHUB_EVENT_PATH" => "${{ github.event_path }}",
  "GITHUB_REF" => "${{ github.ref }}",
  "GITHUB_REPOSITORY" => "${{ github.repository }}",
  "GITHUB_REPOSITORY_ID" => "${{ github.repository_id }}"
}, "reconciliation command environment must match the reviewed values exactly")

uses = File.read(workflow_path).scan(/^\s*uses:\s*([^\s#]+)/).flatten
require_check(uses.all? { |reference| reference.match?(/^[^@]+@[0-9a-f]{40}$/) }, "every third-party Action must be pinned to a full commit SHA")
require_check(source.include?('return conclusion == "failure" || conclusion == "timed_out" || conclusion == "startup_failure"'), "failure conclusion policy changed")
require_check(source.include?('conclusion != "success"'), "only a successful attempt may be treated as recovery")
require_check(source.include?("GetWorkflowRunByID"), "workflow_run triggers must re-fetch their triggering run by ID")

puts "actions watcher workflow routing checks passed"
