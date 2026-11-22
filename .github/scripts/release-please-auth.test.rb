#!/usr/bin/env ruby
# frozen_string_literal: true

require "base64"
require "digest"
require "fileutils"
require "json"
require "minitest/autorun"
require "open3"
require "socket"
require "thread"
require "tmpdir"
require "timeout"
require "yaml"

ROOT = File.expand_path("../..", __dir__)
WORKFLOW = File.join(ROOT, ".github/workflows/release-please.yml")
TOKEN = "local-test-release-token-9bf2"
TAG = "v1.3.0"
COMMIT = "0123456789abcdef0123456789abcdef01234567"
REPOSITORY_URL = "https://github.com/orang-gaboets/octostate.git"
REPOSITORY_EXTRAHEADER_KEY = "http.#{REPOSITORY_URL}.extraheader"
REPOSITORY_FOLLOW_REDIRECTS_KEY = "http.#{REPOSITORY_URL}.followredirects"
REPOSITORY_SSL_VERIFY_KEY = "http.#{REPOSITORY_URL}.sslVerify"
ASSET_NAMES = %w[
  octostate_1.3.0_darwin_amd64.tar.gz
  octostate_1.3.0_darwin_arm64.tar.gz
  octostate_1.3.0_linux_amd64.tar.gz
  octostate_1.3.0_linux_arm64.tar.gz
  octostate_1.3.0_windows_amd64.zip
].freeze
ENCODED_TOKEN = Base64.strict_encode64("x-access-token:#{TOKEN}")
EXPECTED_HEADER = "AUTHORIZATION: Basic #{ENCODED_TOKEN}"

class ReleasePleaseAuthTest < Minitest::Test
  def setup
    @tempdirs = []
    @workflow = YAML.safe_load(File.read(WORKFLOW), aliases: true)
    @steps = @workflow.fetch("jobs").fetch("release-please").fetch("steps")
  end

  def teardown
    @tempdirs.each { |path| FileUtils.remove_entry(path) if File.exist?(path) }
  end

  def step(name)
    @steps.find { |candidate| candidate["name"] == name }
  end

  def required_step(name)
    step(name) || flunk("missing workflow step: #{name}")
  end

  def run_step(name, overrides = {}, xtrace: false)
    run_script(required_step(name).fetch("run"), overrides, xtrace: xtrace)
  end

  def run_script(script, overrides = {}, xtrace: false)
    root = Dir.mktmpdir("release-please-auth-")
    @tempdirs << root
    fixture = prepare_fixture(root)
    if overrides["TEST_TRUNCATE_CHECKSUMS"] == "true"
      manifest = File.join(fixture.fetch(:dist_dir), "checksums.txt")
      File.write(manifest, File.readlines(manifest).drop(1).join)
    end
    mapfile_compat = <<~'BASH'
      mapfile() {
        local array_name="${2:-}" line
        [[ "${1:-}" == "-t" && "${array_name}" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]] || return 2
        while IFS= read -r line; do
          eval "${array_name}+=(\"\$line\")"
        done
      }
    BASH
    script = "#{mapfile_compat}\n#{script}"
    args = xtrace ? ["bash", "-x", "-c", script] : ["bash", "-c", script]
    env = fixture.fetch(:env).merge(overrides)
    stdout, stderr, status = Open3.capture3(env, *args, chdir: root)
    fixture.merge(root: root, stdout: stdout, stderr: stderr, status: status)
  end

  def prepare_fixture(root)
    stub_bin = File.join(root, "bin")
    dist_dir = File.join(root, "dist")
    git_dir = File.join(root, ".git")
    FileUtils.mkdir_p([stub_bin, dist_dir, git_dir])

    ASSET_NAMES.each do |name|
      File.binwrite(File.join(dist_dir, name), "fixture artifact #{name}\n")
    end
    checksums = ASSET_NAMES.map do |name|
      "#{Digest::SHA256.file(File.join(dist_dir, name)).hexdigest}  #{name}"
    end
    File.write(File.join(dist_dir, "checksums.txt"), checksums.join("\n") + "\n")

    global_config = File.join(root, "global.gitconfig")
    system_config = File.join(root, "system.gitconfig")
    local_config = File.join(git_dir, "config")
    global_contents = <<~GITCONFIG
      [http "https://github.com"]
        extraheader = AUTHORIZATION: Basic inherited-global
        sslVerify = false
      [http "#{REPOSITORY_URL}"]
        sslVerify = false
    GITCONFIG
    local_contents = <<~GITCONFIG
      [http "https://github.com"]
        extraheader = AUTHORIZATION: Basic inherited-local
        sslVerify = false
      [http "#{REPOSITORY_URL}"]
        sslVerify = false
    GITCONFIG
    File.write(global_config, global_contents)
    File.write(system_config, "")
    File.write(local_config, local_contents)

    write_git_stub(stub_bin)
    write_gh_stub(stub_bin)
    write_sha256sum_stub(stub_bin)

    expected_assets = (ASSET_NAMES + ["checksums.txt"]).join("\n")
    {
      root: root,
      dist_dir: dist_dir,
      global_config: global_config,
      local_config: local_config,
      global_contents: global_contents,
      local_contents: local_contents,
      git_log: File.join(root, "git-records.jsonl"),
      gh_log: File.join(root, "gh-records.jsonl"),
      gh_api_count: File.join(root, "gh-api-count"),
      release_edit: File.join(root, "release-edit-called"),
      output_path: File.join(root, "github-output"),
      env: {
        "PATH" => "#{stub_bin}:#{ENV.fetch("PATH", "")}",
        "GITHUB_SERVER_URL" => "https://github.com",
        "GITHUB_API_URL" => "https://api.github.com",
        "GITHUB_REPOSITORY" => "orang-gaboets/octostate",
        "DISPATCH_REF" => "refs/heads/main",
        "VERIFY_GIT_AUTH" => "false",
        "RETRY_TAG" => "",
        "RETRY_COMMIT" => "",
        "GH_TOKEN" => TOKEN,
        "RELEASE_CREDENTIAL_SOURCE" => "release-app",
        "RELEASE_TAG" => TAG,
        "EXPECTED_COMMIT" => COMMIT,
        "TEST_HEAD_SHA" => COMMIT,
        "TEST_PEELED_SHA" => COMMIT,
        "TEST_LIGHTWEIGHT_SHA" => COMMIT,
        "TEST_BRANCH_SHA" => COMMIT,
        "TEST_GIT_STATUS" => "0",
        "TEST_FETCH_STATUS" => "0",
        "TEST_EMPTY_PEELED" => "false",
        "TEST_DRAFT" => "true",
        "TEST_TAG_NAME" => TAG,
        "TEST_EXTRA_ASSET" => "false",
        "TEST_CORRUPT_ASSET" => "false",
        "TEST_CORRUPT_ASSET_NAME" => ASSET_NAMES.first,
        "TEST_ASSET_NAMES" => expected_assets,
        "TEST_DIST_DIR" => dist_dir,
        "TEST_GIT_LOG" => File.join(root, "git-records.jsonl"),
        "TEST_EXPECTED_GIT_AUTH_HEADER" => EXPECTED_HEADER,
        "TEST_GH_LOG" => File.join(root, "gh-records.jsonl"),
        "TEST_GH_API_COUNT" => File.join(root, "gh-api-count"),
        "TEST_RELEASE_EDIT" => File.join(root, "release-edit-called"),
        "GITHUB_OUTPUT" => File.join(root, "github-output"),
        "GIT_CONFIG_GLOBAL" => global_config,
        "GIT_CONFIG_SYSTEM" => system_config,
        "GIT_SSL_NO_VERIFY" => "1",
        "GIT_CONFIG_COUNT" => "2",
        "GIT_CONFIG_KEY_0" => "http.https://github.com/.extraheader",
        "GIT_CONFIG_VALUE_0" => "AUTHORIZATION: Basic inherited-environment",
        "GIT_CONFIG_KEY_1" => "credential.helper",
        "GIT_CONFIG_VALUE_1" => "inherited-credential-helper",
        "GIT_CONFIG_PARAMETERS" => "'http.https://github.com/.extraheader=AUTHORIZATION: Basic inherited-parameters'",
        "GIT_CURL_VERBOSE" => "1",
        "GIT_TRACE" => "1",
        "GIT_TRACE_CURL" => "1",
        "GIT_TRACE_FSMONITOR" => "1",
        "GIT_TRACE_PACK_ACCESS" => "1",
        "GIT_TRACE_PACKET" => "1",
        "GIT_TRACE_PACKFILE" => "1",
        "GIT_TRACE_PERFORMANCE" => "1",
        "GIT_TRACE_REFS" => "1",
        "GIT_TRACE_SETUP" => "1",
        "GIT_TRACE_SHALLOW" => "1",
        "GIT_TRACE2" => "1",
        "GIT_TRACE2_EVENT" => "1",
        "GIT_TRACE2_PERF" => "1",
        "GIT_TRACE2_CONFIG_PARAMS" => "GIT_CONFIG_PARAMETERS",
        "GIT_TRACE2_ENV_VARS" => "GH_TOKEN",
        "GIT_TRACE_REDACT" => "0"
      }
    }
  end

  def write_git_stub(stub_bin)
    path = File.join(stub_bin, "git")
    File.write(path, <<~'RUBY')
      #!/usr/bin/env ruby
      require "json"

      args = ARGV
      trace_names = %w[
        GIT_TRACE GIT_TRACE_CURL GIT_TRACE_FSMONITOR GIT_TRACE_PACK_ACCESS
        GIT_TRACE_PACKET GIT_TRACE_PACKFILE GIT_TRACE_PERFORMANCE GIT_TRACE_REFS
        GIT_TRACE_SETUP GIT_TRACE_SHALLOW GIT_TRACE2 GIT_TRACE2_EVENT GIT_TRACE2_PERF
        GIT_TRACE2_CONFIG_PARAMS GIT_TRACE2_ENV_VARS GIT_TRACE_REDACT GIT_CURL_VERBOSE
      ]
      record = {
        "args" => args,
        "auth_header_matches" => ENV["OCTOSTATE_GIT_AUTH_HEADER"] == ENV["TEST_EXPECTED_GIT_AUTH_HEADER"],
        "auth_header_present" => !ENV["OCTOSTATE_GIT_AUTH_HEADER"].to_s.empty?,
        "raw_token_present" => ENV.key?("GH_TOKEN"),
        "ssl_no_verify_present" => ENV.key?("GIT_SSL_NO_VERIFY"),
        "config_count" => ENV["GIT_CONFIG_COUNT"],
        "config_keys" => ENV.keys.grep(/\AGIT_CONFIG_KEY_/),
        "config_values" => ENV.keys.grep(/\AGIT_CONFIG_VALUE_/),
        "config_parameters_present" => ENV.key?("GIT_CONFIG_PARAMETERS"),
        "trace_values" => trace_names.to_h { |name| [name, ENV[name]] }
      }
      File.open(ENV.fetch("TEST_GIT_LOG"), "a") do |file|
        file.puts(JSON.generate(record))
      end

      if args.include?("fetch")
        exit ENV.fetch("TEST_FETCH_STATUS", "0").to_i
      end
      if args.include?("rev-parse")
        puts ENV.fetch("TEST_HEAD_SHA", ENV.fetch("EXPECTED_COMMIT", ""))
        exit 0
      end
      if args.include?("ls-remote")
        status = ENV.fetch("TEST_GIT_STATUS", "0").to_i
        if status != 0
          warn ENV.fetch("TEST_GIT_ERROR", "fatal: simulated transport error")
          exit status
        end
        ref = args.last.to_s
        if ref.end_with?("^{}") && ENV.fetch("TEST_EMPTY_PEELED", "false") == "true"
          exit 0
        end
        if ref.include?("refs/heads/")
          sha = ENV.fetch("TEST_BRANCH_SHA", ENV.fetch("EXPECTED_COMMIT", ""))
          ref = ENV.fetch("TEST_BRANCH_REF", ref)
        elsif ref.end_with?("^{}")
          sha = ENV.fetch("TEST_PEELED_SHA", ENV.fetch("EXPECTED_COMMIT", ""))
        else
          sha = ENV.fetch("TEST_LIGHTWEIGHT_SHA", ENV.fetch("EXPECTED_COMMIT", ""))
        end
        state_path = ENV.fetch("TEST_GH_API_COUNT")
        if File.file?(state_path) && JSON.parse(File.read(state_path))["reads"].to_s == ENV.fetch("TEST_DRIFT_AT", "never")
          sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        end
        puts "#{sha}\t#{ref}"
        exit 0
      end
      exit 0
    RUBY
    FileUtils.chmod(0o755, path)
  end

  def write_gh_stub(stub_bin)
    path = File.join(stub_bin, "gh")
    File.write(path, <<~'RUBY')
      #!/usr/bin/env ruby
      require "json"

      args = ARGV
      File.open(ENV.fetch("TEST_GH_LOG"), "a") { |f| f.puts(JSON.generate(args)) }
      abort "unexpected gh invocation" unless args.first == "api"
      endpoint = args.find { |arg| arg.start_with?("repos/", "https://") }.to_s
      repo = "repos/orang-gaboets/octostate"
      state_path = ENV.fetch("TEST_GH_API_COUNT")
      state = if File.file?(state_path)
        JSON.parse(File.read(state_path))
      else
        initial = ENV.fetch("TEST_PARTIAL", "false") == "true" ? ENV.fetch("TEST_ASSET_NAMES").split("\n").first(2) : []
        { "reads" => 0, "assets" => initial.each_with_index.map { |name, i| { "name" => name, "id" => i + 1, "state" => "uploaded" } } }
      end
      save = -> { File.write(state_path, JSON.generate(state)) }
      fail_at = ENV.fetch("TEST_API_FAILURE", "")
      if endpoint.include?("/releases/tags/")
        warn "gh: Not Found (HTTP 404)"
        exit 1
      elsif endpoint == "#{repo}/releases?per_page=100"
        abort "discovery error" if fail_at == "discovery"
        count = Integer(ENV.fetch("TEST_MATCH_COUNT", "1"))
        releases = Array.new(count) { |i| { "id" => 42 + i, "tag_name" => ENV.fetch("TEST_TAG_NAME"), "draft" => ENV.fetch("TEST_DRAFT") == "true" } }
        puts JSON.generate([ENV["TEST_MALFORMED_DISCOVERY"] == "true" ? {} : [], releases])
      elsif endpoint == "#{repo}/releases/42" && !args.include?("PATCH")
        state["reads"] += 1
        save.call
        abort "release metadata error" if fail_at == "metadata#{state['reads']}"
        change = ENV.fetch("TEST_CHANGE_AT", "").split(":")
        tag = change == [state["reads"].to_s, "tag"] ? "wrong-tag" : ENV.fetch("TEST_TAG_NAME")
        id = change == [state["reads"].to_s, "id"] ? 99 : 42
        draft = change == [state["reads"].to_s, "draft"] ? false : ENV.fetch("TEST_DRAFT") == "true"
        url = ENV.fetch("TEST_UPLOAD_URL", "https://uploads.github.com/repos/orang-gaboets/octostate/releases/42/assets{?name,label}")
        puts JSON.generate({ "id" => id, "tag_name" => tag, "draft" => draft,
          "url" => "https://api.github.com/#{repo}/releases/42", "upload_url" => url })
      elsif endpoint == "#{repo}/releases/42/assets?per_page=100"
        abort "asset list error" if fail_at == "assets#{state['reads']}"
        if ENV.fetch("TEST_MALFORMED_AT", "") == state["reads"].to_s
          puts "[null]"
          exit 0
        end
        assets = state.fetch("assets").map(&:dup)
        assets << { "name" => "unexpected.bin", "id" => 999, "state" => "uploaded" } if ENV["TEST_EXTRA_ASSET"] == "true"
        assets << assets.first.dup if ENV["TEST_DUPLICATE_ASSET"] == "true" && !assets.empty?
        assets.pop if ENV.fetch("TEST_MISSING_AT", "") == state["reads"].to_s
        assets.first["id"] = 777 if ENV["TEST_REPLACE_ASSET"] == "true" && state["reads"] == 3 && !assets.empty?
        assets.first["state"] = "starter" if ENV["TEST_STARTER_ASSET"] == "true" && state["reads"] > 1 && !assets.empty?
        puts JSON.generate([assets.take(2), assets.drop(2)])
      elsif endpoint.match?(%r{\A#{repo}/releases/assets/\d+\z}) && args.include?("DELETE")
        abort "delete error" if fail_at == "delete"
        id = endpoint.split("/").last.to_i
        abort "unknown asset" unless state["assets"].any? { |a| a["id"] == id }
        state["assets"].reject! { |a| a["id"] == id }
        save.call
      elsif endpoint.match?(%r{\A#{repo}/releases/assets/\d+\z}) && args.include?("Accept: application/octet-stream")
        abort "download error" if fail_at == "download"
        asset = state.fetch("assets").find { |a| a["id"] == endpoint.split("/").last.to_i }
        abort "unknown asset" unless asset
        body = File.binread(File.join(ENV.fetch("TEST_DIST_DIR"), asset.fetch("name")))
        body += "corrupt" if ENV["TEST_CORRUPT_ASSET"] == "true" && asset["name"] == ENV["TEST_CORRUPT_ASSET_NAME"]
        STDOUT.write(body)
      elsif endpoint == "https://uploads.github.com/#{repo}/releases/42/assets" && args.include?("POST")
        abort "upload error" if fail_at == "upload"
        name_arg = args.find { |arg| arg.start_with?("name=") }
        name = name_arg&.delete_prefix("name=")
        input = args[args.index("--input").to_i + 1]
        abort "bad upload" unless name && File.basename(input) == name && File.file?(input)
        abort "duplicate asset" if state["assets"].any? { |asset| asset["name"] == name }
        state["assets"] << { "name" => name, "id" => 100 + state["assets"].length, "state" => "uploaded" }
        save.call
        puts "{}"
      elsif endpoint == "#{repo}/releases/42" && args.include?("PATCH")
        abort "bad publication payload" unless args.include?("draft=false") && args.include?("tag_name=#{ENV.fetch('RELEASE_TAG')}")
        File.write(ENV.fetch("TEST_RELEASE_EDIT"), "called")
      else
        abort "unexpected gh endpoint: #{endpoint}"
      end
    RUBY
    FileUtils.chmod(0o755, path)
  end

  def write_sha256sum_stub(stub_bin)
    path = File.join(stub_bin, "sha256sum")
    File.write(path, <<~'RUBY')
      #!/usr/bin/env ruby
      require "digest"

      unless ARGV.first == "-c" && ARGV.length == 2
        warn "unsupported sha256sum fixture operation"
        exit 99
      end

      failed = false
      File.foreach(ARGV.fetch(1)) do |line|
        expected, filename = line.chomp.split(/\s+/, 2)
        if filename && File.file?(filename) && Digest::SHA256.file(filename).hexdigest == expected
          puts "#{filename}: OK"
        else
          warn "#{filename || ARGV.fetch(1)}: FAILED"
          failed = true
        end
      end
      exit(failed ? 1 : 0)
    RUBY
    FileUtils.chmod(0o755, path)
  end

  def git_records(result)
    path = result.fetch(:git_log)
    return [] unless File.file?(path)

    File.readlines(path, chomp: true).map { |line| JSON.parse(line) }
  end

  def gh_records(result)
    path = result.fetch(:gh_log)
    return [] unless File.file?(path)

    File.readlines(path, chomp: true).map { |line| JSON.parse(line) }
  end

  def assert_basic_git_record(record)
    assert_equal true, record.fetch("auth_header_matches")
    assert_equal true, record.fetch("auth_header_present")
    refute record.fetch("raw_token_present")
    assert_equal "0", record.fetch("config_count")
    assert_empty record.fetch("config_keys")
    assert_empty record.fetch("config_values")
    refute record.fetch("config_parameters_present")
    refute record.fetch("ssl_no_verify_present")
    trace_values = record.fetch("trace_values")
    %w[
      GIT_TRACE GIT_TRACE_CURL GIT_TRACE_FSMONITOR GIT_TRACE_PACK_ACCESS
      GIT_TRACE_PACKET GIT_TRACE_PACKFILE GIT_TRACE_PERFORMANCE GIT_TRACE_REFS
      GIT_TRACE_SETUP GIT_TRACE_SHALLOW GIT_TRACE2 GIT_TRACE2_EVENT GIT_TRACE2_PERF
    ].each { |name| assert_equal "0", trace_values.fetch(name), "#{name} must be disabled" }
    assert_equal "1", trace_values.fetch("GIT_TRACE_REDACT")
    assert_nil trace_values.fetch("GIT_CURL_VERBOSE")
    assert_nil trace_values.fetch("GIT_TRACE2_CONFIG_PARAMS")
    assert_nil trace_values.fetch("GIT_TRACE2_ENV_VARS")
    args = record.fetch("args")
    refute_includes args.join("\n"), TOKEN
    refute_includes args.join("\n"), ENCODED_TOKEN
    assert_includes args, "-c"
    assert_includes args, "#{REPOSITORY_EXTRAHEADER_KEY}="
    assert_includes args, "--config-env=#{REPOSITORY_EXTRAHEADER_KEY}=OCTOSTATE_GIT_AUTH_HEADER"
    assert_includes args, "#{REPOSITORY_FOLLOW_REDIRECTS_KEY}=false"
    assert_includes args, "#{REPOSITORY_SSL_VERIFY_KEY}=true"
    assert_includes args, "credential.helper="
    assert_includes args, "credential.interactive=false"
  end

  def assert_configs_unchanged(result)
    assert_equal result.fetch(:global_contents), File.read(result.fetch(:global_config))
    assert_equal result.fetch(:local_contents), File.read(result.fetch(:local_config))
  end

  def assert_no_release_edit(result)
    refute File.exist?(result.fetch(:release_edit)), "release edit ran after a failed gate"
  end

  def test_checkout_fetch_uses_basic_auth_and_preserves_tag_commit_identity
    result = run_step("Check out the exact release tag", {}, xtrace: true)
    assert_predicate result.fetch(:status), :success?, result.fetch(:stderr)

    records = git_records(result)
    fetch = records.find { |record| record.fetch("args").include?("fetch") }
    refute_nil fetch
    setup_git = records.take_while { |record| !record.fetch("args").include?("fetch") }
    assert_equal %w[init remote], setup_git.map { |record| record.fetch("args").first }
    setup_git.each do |record|
      refute record.fetch("raw_token_present"), "GH_TOKEN must not be passed to pre-fetch Git commands"
    end
    assert_basic_git_record(fetch)
    assert_includes fetch.fetch("args"), "refs/tags/#{TAG}:refs/tags/#{TAG}"
    assert records.any? { |record| record.fetch("args").include?("--detach") && record.fetch("args").include?(TAG) }
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), ENCODED_TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), EXPECTED_HEADER
    assert_configs_unchanged(result)
  end

  def test_checkout_stops_on_auth_or_transport_failure_before_checkout
    result = run_step("Check out the exact release tag", { "TEST_FETCH_STATUS" => "128" })
    refute_predicate result.fetch(:status), :success?
    refute git_records(result).any? { |record| record.fetch("args").include?("--detach") }
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), ENCODED_TOKEN
  end

  def test_checkout_rejects_expected_commit_mismatch
    result = run_step("Check out the exact release tag", { "TEST_HEAD_SHA" => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" })
    refute_predicate result.fetch(:status), :success?
    assert_includes result.fetch(:stderr), "does not target expected commit"
  end

  def test_upload_recheck_uses_basic_auth_and_allows_lightweight_tag_fallback
    result = run_step("Upload and verify release artifacts", { "TEST_EMPTY_PEELED" => "true" }, xtrace: true)
    assert_predicate result.fetch(:status), :success?, result.fetch(:stderr)

    calls = git_records(result).select { |record| record.fetch("args").include?("ls-remote") }
    assert_equal 6, calls.length
    calls.each { |record| assert_basic_git_record(record) }
    assert_equal ["refs/tags/#{TAG}^{}", "refs/tags/#{TAG}"] * 3, calls.map { |call| call.fetch("args").last }
    assert File.exist?(result.fetch(:release_edit)), "verified draft release was not published"
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), ENCODED_TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), EXPECTED_HEADER
    assert_configs_unchanged(result)
  end

  def test_upload_finds_draft_when_tag_endpoint_returns_404
    result = run_step("Upload and verify release artifacts")

    assert_predicate result.fetch(:status), :success?, result.fetch(:stderr)
    assert File.exist?(result.fetch(:release_edit)), "verified draft release was not published"
    calls = gh_records(result)
    assert_equal 3, calls.count { |call| call.include?("repos/orang-gaboets/octostate/releases/42") && !call.include?("PATCH") }
    assert_equal 6, calls.count { |call| call.include?("POST") && call.include?("https://uploads.github.com/repos/orang-gaboets/octostate/releases/42/assets") }
    refute calls.flatten.any? { |arg| arg.include?("/releases/tags/") }
  end

  def test_upload_fails_closed_for_invalid_discovery_and_metadata
    cases = [
      { "TEST_MATCH_COUNT" => "0" }, { "TEST_MATCH_COUNT" => "2" },
      { "TEST_TAG_NAME" => "wrong-tag" }, { "TEST_DRAFT" => "false" },
      { "TEST_MALFORMED_DISCOVERY" => "true" },
      { "TEST_API_FAILURE" => "discovery" }, { "TEST_UPLOAD_URL" => "https://attacker.invalid/upload{?name,label}" }
    ]
    (1..3).each do |boundary|
      %w[id tag draft].each { |field| cases << { "TEST_CHANGE_AT" => "#{boundary}:#{field}" } }
      cases << { "TEST_API_FAILURE" => "metadata#{boundary}" }
      cases << { "TEST_API_FAILURE" => "assets#{boundary}" }
      cases << { "TEST_DRIFT_AT" => boundary.to_s }
      cases << { "TEST_MALFORMED_AT" => boundary.to_s }
    end
    cases.each do |scenario|
      result = run_step("Upload and verify release artifacts", scenario)
      refute_predicate result.fetch(:status), :success?, scenario.inspect
      assert_no_release_edit(result)
    end
  end

  def test_upload_retries_only_matching_partial_assets
    result = run_step("Upload and verify release artifacts", { "TEST_PARTIAL" => "true" })

    assert_predicate result.fetch(:status), :success?, result.fetch(:stderr)
    deleted = gh_records(result).select { |call| call.include?("DELETE") }.map(&:last)
    assert_equal %w[repos/orang-gaboets/octostate/releases/assets/1 repos/orang-gaboets/octostate/releases/assets/2], deleted
    assert File.exist?(result.fetch(:release_edit))
  end

  def test_upload_rejects_manifest_missing_an_archive_before_mutation
    result = run_step("Upload and verify release artifacts", {
      "TEST_TRUNCATE_CHECKSUMS" => "true", "TEST_CORRUPT_ASSET" => "true"
    })

    refute_predicate result.fetch(:status), :success?
    assert_no_release_edit(result)
    refute gh_records(result).any? { |call| call.include?("DELETE") || call.include?("POST") }
  end

  def test_upload_refuses_unverified_or_changed_assets_and_api_errors
    scenarios = [
      { "TEST_EXTRA_ASSET" => "true", "TEST_PARTIAL" => "true" },
      { "TEST_DUPLICATE_ASSET" => "true", "TEST_PARTIAL" => "true" },
      { "TEST_REPLACE_ASSET" => "true" }, { "TEST_STARTER_ASSET" => "true" },
      { "TEST_MISSING_AT" => "2" }, { "TEST_MISSING_AT" => "3" },
      { "TEST_API_FAILURE" => "delete", "TEST_PARTIAL" => "true" },
      { "TEST_API_FAILURE" => "upload" }, { "TEST_API_FAILURE" => "download" },
      { "TEST_CORRUPT_ASSET" => "true", "TEST_CORRUPT_ASSET_NAME" => "checksums.txt" }
    ]
    scenarios.each do |scenario|
      result = run_step("Upload and verify release artifacts", scenario)
      refute_predicate result.fetch(:status), :success?, scenario.inspect
      assert_no_release_edit(result)
      if scenario.key?("TEST_EXTRA_ASSET") || scenario.key?("TEST_DUPLICATE_ASSET")
        refute gh_records(result).any? { |call| call.include?("DELETE") || call.include?("POST") }
      end
    end
  end

  def test_upload_authentication_failure_stops_before_publication
    result = run_step(
      "Upload and verify release artifacts",
      { "TEST_GIT_STATUS" => "128", "TEST_GIT_ERROR" => "fatal: Authentication failed" }
    )
    refute_predicate result.fetch(:status), :success?
    assert_no_release_edit(result)
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), ENCODED_TOKEN
  end

  def test_upload_transport_failure_stops_before_publication
    result = run_step(
      "Upload and verify release artifacts",
      { "TEST_GIT_STATUS" => "128", "TEST_GIT_ERROR" => "fatal: unable to access: Could not resolve host" }
    )
    refute_predicate result.fetch(:status), :success?
    assert_no_release_edit(result)
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), ENCODED_TOKEN
  end

  def test_upload_rejects_moved_remote_tag_before_publication
    result = run_step(
      "Upload and verify release artifacts",
      { "TEST_PEELED_SHA" => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
    )
    refute_predicate result.fetch(:status), :success?
    assert_includes result.fetch(:stderr), "release tag #{TAG} moved before publication"
    assert_no_release_edit(result)
  end

  def test_upload_rejects_unexpected_assets_before_publication
    result = run_step(
      "Upload and verify release artifacts",
      { "TEST_EXTRA_ASSET" => "true" }
    )
    refute_predicate result.fetch(:status), :success?
    assert_includes result.fetch(:stderr), "unexpected or duplicate assets"
    assert_no_release_edit(result)
  end

  def test_upload_rejects_non_draft_release_before_publication
    result = run_step(
      "Upload and verify release artifacts",
      { "TEST_DRAFT" => "false" }
    )
    refute_predicate result.fetch(:status), :success?
    assert_includes result.fetch(:stderr), "draft"
    assert_no_release_edit(result)
  end

  def test_upload_rejects_corrupted_downloaded_asset_before_publication
    result = run_step(
      "Upload and verify release artifacts",
      { "TEST_CORRUPT_ASSET" => "true" }
    )

    refute_predicate result.fetch(:status), :success?
    assert_includes result.fetch(:stderr), "#{ASSET_NAMES.first}: FAILED"
    assert_no_release_edit(result)
  end

  def test_verifier_is_main_only_read_only_and_separate_from_packaging
    trigger = @workflow["on"] || @workflow[true]
    input = trigger.fetch("workflow_dispatch").fetch("inputs").fetch("verify_git_auth")
    assert_equal "boolean", input.fetch("type")
    assert_equal false, input.fetch("default")

    validate = required_step("Validate retry inputs")
    assert_equal "github.event_name == 'workflow_dispatch'", validate.fetch("if")
    assert_operator @steps.index(validate), :<, @steps.index(required_step("Generate GitHub App token"))
    valid = run_step(
      "Validate retry inputs",
      { "VERIFY_GIT_AUTH" => "true", "DISPATCH_REF" => "refs/heads/main" }
    )
    assert_predicate valid.fetch(:status), :success?, valid.fetch(:stderr)

    verifier = required_step("Verify Release Please Git authentication")
    assert_equal(
      "${{ steps.app-token.outputs.token != '' && 'release-app' || 'GITHUB_TOKEN' }}",
      verifier.fetch("env").fetch("RELEASE_CREDENTIAL_SOURCE")
    )
    result = run_step(verifier.fetch("name"))
    assert_predicate result.fetch(:status), :success?, result.fetch(:stderr)
    assert_includes result.fetch(:stdout), "GitHub Git authentication check passed using release-app."
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), ENCODED_TOKEN
    refute_includes result.fetch(:stdout) + result.fetch(:stderr), EXPECTED_HEADER
    calls = git_records(result)
    assert_equal 1, calls.length
    assert_basic_git_record(calls.first)
    assert_includes calls.first.fetch("args"), "ls-remote"
    assert_includes calls.first.fetch("args"), "--exit-code"
    assert_equal "refs/heads/main", calls.first.fetch("args").last
    assert_empty gh_records(result)
    refute File.exist?(result.fetch(:release_edit))

    fallback = run_step(verifier.fetch("name"), { "RELEASE_CREDENTIAL_SOURCE" => "GITHUB_TOKEN" })
    assert_predicate fallback.fetch(:status), :success?, fallback.fetch(:stderr)
    assert_includes fallback.fetch(:stdout), "GitHub Git authentication check passed using GITHUB_TOKEN."
    refute_includes fallback.fetch(:stdout) + fallback.fetch(:stderr), TOKEN
    refute_includes fallback.fetch(:stdout) + fallback.fetch(:stderr), ENCODED_TOKEN
    refute_includes fallback.fetch(:stdout) + fallback.fetch(:stderr), EXPECTED_HEADER

    release_please = required_step("Run release-please")
    assert_equal "github.event_name != 'workflow_dispatch' || (inputs.release_tag == '' && inputs.verify_git_auth != true)", release_please.fetch("if")
    assert_equal "github.event_name == 'workflow_dispatch' && inputs.verify_git_auth == true", verifier.fetch("if")
    package_steps = @steps.select do |candidate|
      ["Check out the exact release tag", "Build release artifacts", "Upload and verify release artifacts"].include?(candidate["name"])
    end
    package_steps.each do |candidate|
      assert_equal "steps.release.outputs.should_package == 'true'", candidate.fetch("if")
    end

    output = run_step(
      "Resolve release to package",
      {
        "CREATED_TAG" => "",
        "CREATED_COMMIT" => "",
        "RETRY_TAG" => "",
        "RETRY_COMMIT" => ""
      }
    )
    assert_predicate output.fetch(:status), :success?, output.fetch(:stderr)
    assert_includes File.read(output.fetch(:output_path)), "should_package=false"
  end

  def test_verifier_rejects_non_main_ref_and_retry_inputs
    not_main = run_step(
      "Validate retry inputs",
      { "VERIFY_GIT_AUTH" => "true", "DISPATCH_REF" => "refs/heads/feature/test" }
    )
    refute_predicate not_main.fetch(:status), :success?

    with_retry = run_step(
      "Validate retry inputs",
      {
        "VERIFY_GIT_AUTH" => "true",
        "DISPATCH_REF" => "refs/heads/main",
        "RETRY_TAG" => TAG,
        "RETRY_COMMIT" => COMMIT
      }
    )
    refute_predicate with_retry.fetch(:status), :success?
  end

  def test_verifier_rejects_nested_ref_matching_main_suffix
    result = run_step(
      "Verify Release Please Git authentication",
      { "TEST_BRANCH_REF" => "refs/heads/feature/refs/heads/main" }
    )

    refute_predicate result.fetch(:status), :success?
    assert_includes result.fetch(:stderr), "did not return refs/heads/main"
  end

  def test_retry_inputs_must_be_supplied_together
    tag_without_commit = run_step(
      "Validate retry inputs",
      { "RETRY_TAG" => TAG, "RETRY_COMMIT" => "" }
    )
    refute_predicate tag_without_commit.fetch(:status), :success?

    commit_without_tag = run_step(
      "Validate retry inputs",
      { "RETRY_TAG" => "", "RETRY_COMMIT" => COMMIT }
    )
    refute_predicate commit_without_tag.fetch(:status), :success?
  end

  def test_token_fallback_and_workflow_permissions_remain_unchanged
    names = [
      "Check out the exact release tag",
      "Upload and verify release artifacts",
      "Verify Release Please Git authentication"
    ]
    names.each do |name|
      expression = required_step(name).fetch("env").fetch("GH_TOKEN")
      assert_match(/steps\.app-token\.outputs\.token \|\| secrets\.GITHUB_TOKEN/, expression)
    end

    assert_equal(
      { "contents" => "write", "issues" => "write", "pull-requests" => "write" },
      @workflow.fetch("permissions")
    )
    token_step = required_step("Generate GitHub App token")
    assert_equal "write", token_step.fetch("with").fetch("permission-contents")
    assert_equal "write", token_step.fetch("with").fetch("permission-issues")
    assert_equal "write", token_step.fetch("with").fetch("permission-pull-requests")
  end

  def test_workflow_shell_blocks_parse_as_bash
    names = [
      "Check out the exact release tag",
      "Upload and verify release artifacts",
      "Verify Release Please Git authentication",
      "Validate retry inputs"
    ]
    names.each do |name|
      script = required_step(name).fetch("run")
      _stdout, stderr, status = Open3.capture3("bash", "-n", stdin_data: script)
      assert_predicate status, :success?, "#{name}: #{stderr}"
    end
  end

  def test_real_gh_upload_sends_authenticated_binary_with_encoded_name
    root = Dir.mktmpdir("release-please-gh-upload-")
    @tempdirs << root
    asset = File.join(root, "asset.bin")
    File.binwrite(asset, "binary\x00payload\n")
    server = TCPServer.new("127.0.0.1", 0)
    port = server.addr[1]
    requests = Queue.new
    server_thread = Thread.new do
      client = Timeout.timeout(10) { server.accept }
      request = client.gets
      headers = {}
      while (line = client.gets) && line != "\r\n"
        key, value = line.split(":", 2)
        headers[key.downcase] = value.strip
      end
      body = client.read(Integer(headers.fetch("content-length")))
      requests << [request, headers, body]
      client.write("HTTP/1.1 201 Created\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}")
      client.close
    ensure
      server.close unless server.closed?
    end
    stdout, stderr, status = Open3.capture3(
      { "GH_HOST" => "github.com", "GH_TOKEN" => TOKEN, "HTTP_PROXY" => "http://127.0.0.1:#{port}",
        "http_proxy" => "http://127.0.0.1:#{port}", "NO_PROXY" => "", "no_proxy" => "" },
      "gh", "api", "--method", "POST", "--input", asset, "-H", "Content-Type: application/octet-stream",
      "-f", "name=asset+name.bin", "http://uploads.github.com/repos/owner/repo/releases/42/assets"
    )
    assert server_thread.join(10), "gh upload did not reach the loopback server: #{stderr}"
    server_thread.value
    assert_predicate status, :success?, stderr
    request, headers, body = requests.pop
    assert_equal "POST http://uploads.github.com/repos/owner/repo/releases/42/assets?name=asset%2Bname.bin HTTP/1.1\r\n", request
    assert_equal "application/octet-stream", headers.fetch("content-type")
    assert_equal "token #{TOKEN}", headers.fetch("authorization")
    assert_equal "binary\x00payload\n", body
    refute_includes stdout + stderr, TOKEN
  ensure
    server&.close unless server&.closed?
    server_thread&.kill
  end

  def test_config_env_command_replaces_inherited_extraheaders_without_persisting_them
    root = Dir.mktmpdir("release-please-git-config-")
    @tempdirs << root
    global = File.join(root, "global.gitconfig")
    system = File.join(root, "system.gitconfig")
    local = File.join(root, ".git", "config")
    request_headers = Queue.new
    FileUtils.mkdir_p(File.dirname(local))
    server = TCPServer.new("127.0.0.1", 0)
    port = server.addr[1]
    url = "http://127.0.0.1:#{port}"
    repository_url = "#{url}/repo.git"
    config_key = "http.#{repository_url}.extraheader"
    follow_redirects_key = "http.#{repository_url}.followredirects"
    global_contents = <<~GITCONFIG
      [http "#{url}"]
        extraheader = AUTHORIZATION: Basic inherited-global-root
      [http "#{repository_url}"]
        extraheader = AUTHORIZATION: Basic inherited-global-path
        followredirects = true
    GITCONFIG
    File.write(global, global_contents)
    File.write(system, "")
    _stdout, stderr, status = Open3.capture3("git", "init", "--quiet", root)
    assert_predicate status, :success?, stderr
    _stdout, stderr, status = Open3.capture3(
      "git", "config", "--local", "--add", config_key,
      "AUTHORIZATION: Basic inherited-local", chdir: root
    )
    assert_predicate status, :success?, stderr
    _stdout, stderr, status = Open3.capture3(
      "git", "config", "--local", "--add", follow_redirects_key, "true", chdir: root
    )
    assert_predicate status, :success?, stderr
    local_contents = File.read(local)

    server_thread = Thread.new do
      client = Timeout.timeout(5) { server.accept }
      request = client.gets
      while (line = client.gets) && line != "\r\n"
        request << line
      end
      request_headers << request
      client.write("HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
      client.close
    ensure
      server.close unless server.closed?
    end

    stdout, stderr, status = Open3.capture3(
      {
        "GIT_CONFIG_GLOBAL" => global,
        "GIT_CONFIG_SYSTEM" => system,
        "GIT_CONFIG_COUNT" => "0",
        "OCTOSTATE_GIT_AUTH_HEADER" => EXPECTED_HEADER,
        "NO_PROXY" => "127.0.0.1,localhost",
        "no_proxy" => "127.0.0.1,localhost"
      },
      "git",
      "-c",
      "#{config_key}=",
      "--config-env=#{config_key}=OCTOSTATE_GIT_AUTH_HEADER",
      "-c",
      "#{follow_redirects_key}=false",
      "-c",
      "credential.helper=",
      "-c",
      "credential.interactive=false",
      "ls-remote",
      repository_url,
      chdir: root
    )
    assert server_thread.join(5), "loopback Git request did not complete"
    server_thread.value
    refute_predicate status, :success?, "the fixture server must reject Git after recording headers"
    headers = request_headers.pop
    refute_nil headers, "Git did not reach the loopback fixture server: #{stderr}"
    assert_includes headers, EXPECTED_HEADER
    refute_includes headers, "inherited-global-root"
    refute_includes headers, "inherited-global-path"
    refute_includes headers, "inherited-local"
    refute_includes stdout + stderr, TOKEN
    refute_includes stdout + stderr, ENCODED_TOKEN
    refute_includes stdout + stderr, EXPECTED_HEADER
    assert_equal global_contents, File.read(global)
    assert_equal local_contents, File.read(local)

    stdout, stderr, status = Open3.capture3(
      {
        "GIT_CONFIG_GLOBAL" => global,
        "GIT_CONFIG_SYSTEM" => system,
        "GIT_CONFIG_COUNT" => "0"
      },
      "git",
      "-c",
      "#{follow_redirects_key}=false",
      "config",
      "--get-urlmatch",
      "http.followredirects",
      "#{repository_url}/info/refs",
      chdir: root
    )
    assert_predicate status, :success?, stderr
    assert_equal "false\n", stdout
  ensure
    server.close if server && !server.closed?
    server_thread&.kill if server_thread&.alive?
  end
end
