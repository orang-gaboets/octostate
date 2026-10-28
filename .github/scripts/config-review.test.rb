require "fileutils"
require "json"
require "minitest/autorun"
require "open3"
require "rbconfig"
require "tmpdir"
require "yaml"

class ConfigReviewWorkflowTest < Minitest::Test
  ROOT = File.expand_path("../..", __dir__)
  WORKFLOW = YAML.safe_load(File.read(File.join(ROOT, ".github/workflows/config-review.yml")), aliases: true)
  JOB = WORKFLOW.fetch("jobs").fetch("config-review")
  SECRET = "synthetic-organization-token"
  ORIGIN = "https://github.com/orang-gaboets/octostate.git"

  def setup
    @tmp = Dir.mktmpdir("octostate config review ")
    @caller = File.join(@tmp, "caller")
    @runner = File.join(@tmp, "runner temp")
    @stubs = File.join(@tmp, "stubs")
    @source = File.join(@tmp, "source")
    [@caller, @runner, @stubs, @source].each { |p| FileUtils.mkdir_p(p) }
    @git = Open3.capture2("which", "git").first.strip
    git("init", "-q", @source)
    File.write(File.join(@source, "go.mod"), "module github.com/orang-gaboets/octostate/v2\n\ngo 1.25.0\n")
    File.write(File.join(@source, "source.txt"), "old\n")
    git("-C", @source, "add", ".")
    git("-C", @source, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "old")
    @old_sha = git("-C", @source, "rev-parse", "HEAD").strip
    git("-C", @source, "tag", "v1.0.0")
    File.write(File.join(@source, "source.txt"), "new\n")
    git("-C", @source, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qam", "new")
    @sha = git("-C", @source, "rev-parse", "HEAD").strip
    File.write(File.join(@caller, "go.work"), "invalid caller workspace\n")
    @inputs = {"octostate_version" => @sha, "config_dir" => "config directory", "expected_org" => "acme"}
    @secret = SECRET
    @env = {
      "PATH" => "#{@stubs}:/usr/bin:/bin", "RUNNER_TEMP" => @runner,
      "GITHUB_PATH" => File.join(@tmp, "github-path"),
      "GITHUB_ENV" => File.join(@tmp, "github-env"),
      "GITHUB_OUTPUT" => File.join(@tmp, "github-output"),
      "GITHUB_STEP_SUMMARY" => File.join(@tmp, "github-summary"),
      "EVENTS" => File.join(@tmp, "events"), "BUILD_STATE" => File.join(@tmp, "build-state"),
      "REAL_GIT" => @git, "SOURCE" => @source,
      "FAKE_CLI" => File.join(@tmp, "fake-octostate")
    }
    write_stubs
    reset_run
  end

  def teardown
    FileUtils.remove_entry(@tmp)
  end

  def git(*args)
    out, err, status = Open3.capture3(@git || "git", *args)
    raise "#{args.inspect}: #{err}" unless status.success?
    out
  end

  def write_stub(name, body, directory: @stubs)
    path = File.join(directory, name)
    File.write(path, "#!#{RbConfig.ruby}\nrequire 'json'\nrequire 'fileutils'\nTOKEN = #{SECRET.inspect}\n#{body}")
    FileUtils.chmod(0755, path)
  end

  def write_stubs
    write_stub("git", <<~'RUBY')
      File.open(ENV.fetch("EVENTS"), "a") { |f| f.puts JSON.generate(tool: "git", args: ARGV, token: ENV.values.include?(TOKEN)) }
      command = ARGV.find { |a| %w[fetch checkout rev-parse].include?(a) }
      exit 21 if command && ENV["FAIL_GIT"] == command
      if command == "rev-parse" && ENV["WRONG_HEAD"]
        puts "0" * 40
        exit
      end
      if command == "fetch"
        abort "unexpected source repository" unless ARGV.include?("https://github.com/orang-gaboets/octostate.git")
        ARGV.map! { |a| a == "https://github.com/orang-gaboets/octostate.git" ? ENV.fetch("SOURCE") : a }
      end
      exec ENV.fetch("REAL_GIT"), *ARGV
    RUBY
    write_stub("go", <<~'RUBY')
      File.open(ENV.fetch("EVENTS"), "a") { |f| f.puts JSON.generate(tool: "go", args: ARGV, cwd: Dir.pwd, gowork: ENV["GOWORK"], token: ENV.values.include?(TOKEN)) }
      exit 22 if ENV["FAIL_GO"] == ARGV.first
      case ARGV.first
      when "build"
        revision = IO.popen([ENV.fetch("REAL_GIT"), "rev-parse", "HEAD"], &:read).strip
        binary = ARGV.fetch(ARGV.index("-o") + 1)
        FileUtils.mkdir_p(File.dirname(binary))
        FileUtils.cp(ENV.fetch("FAKE_CLI"), binary)
        File.write(ENV.fetch("BUILD_STATE"), JSON.generate(revision: revision))
      when "version"
        state = JSON.parse(File.read(ENV.fetch("BUILD_STATE")))
        puts "octostate: go1.25.13"
        puts "\tpath\tgithub.com/orang-gaboets/octostate/v2/cmd/octostate"
        puts "\tbuild\tvcs=git"
        puts "\tbuild\tvcs.revision=#{ENV.fetch("METADATA_REVISION", state.fetch("revision"))}" unless ENV["MISSING_REVISION"]
        puts "\tbuild\tvcs.modified=#{ENV.fetch("DIRTY", "false")}"
      else
        abort "unexpected go command"
      end
    RUBY
    write_stub("fake-octostate", <<~'RUBY', directory: @tmp)
      token = ENV["OCTOSTATE_GITHUB_TOKEN"]
      File.open(ENV.fetch("EVENTS"), "a") { |f| f.puts JSON.generate(tool: "octostate", args: ARGV, token: ENV.values.include?(TOKEN), correct_token: token == TOKEN) }
      if ENV["ASSERT_ENV_AUTH"] && ARGV[1] != "validate"
        abort "environment authentication required" unless token == TOKEN && !ARGV.include?("--token")
      end
      puts '{"status":"ok"}'
      warn "command diagnostic"
      exit 23 if ENV["FAIL_COMMAND"] == ARGV[1]
    RUBY
  end

  def reset_run
    File.write(@env.fetch("EVENTS"), "")
    %w[GITHUB_PATH GITHUB_ENV GITHUB_OUTPUT GITHUB_STEP_SUMMARY].each { |key| File.write(@env.fetch(key), "") }
    @results = []
    @path = @env.fetch("PATH")
  end

  def expand(value)
    return value.to_s unless value.is_a?(String)
    bindings = {"runner.temp" => @runner, "secrets.octostate_token" => @secret}
    @inputs.each { |k, v| bindings["inputs.#{k}"] = v.to_s }
    value.gsub(/\$\{\{\s*([^}]+?)\s*\}\}/) { bindings.fetch(Regexp.last_match(1)) }
  end

  def run_step(step, overrides = {})
    env = @env.merge("PATH" => @path)
    [JOB.fetch("env", {}), step.fetch("env", {})].each do |mapping|
      mapping.each { |key, value| env[key] = expand(value) }
    end
    out, err, status = Open3.capture3(env.merge(overrides), "bash", "--noprofile", "--norc", "-c", step.fetch("run"), chdir: @caller, unsetenv_others: true)
    @results << {name: step.fetch("name"), out: out, err: err, status: status}
    if status.success?
      @path = ([File.read(@env.fetch("GITHUB_PATH")).lines.map(&:strip), @env.fetch("PATH")].flatten).join(":")
    end
    status
  end

  def run_review(overrides = {})
    JOB.fetch("steps").select { |s| s.key?("run") }.each do |step|
      break unless run_step(step, overrides).success?
    end
    @results.last.fetch(:status)
  end

  def events(tool = nil)
    rows = File.readlines(@env.fetch("EVENTS")).map { |line| JSON.parse(line) }
    tool ? rows.select { |row| row.fetch("tool") == tool } : rows
  end

  def diagnostic
    @results.map { |r| r[:out] + r[:err] }.join
  end

  def test_fetches_and_builds_exact_commit_outside_caller_workspace
    assert run_review.success?, diagnostic
    fetch = events("git").find { |e| e["args"].include?("fetch") }
    refute_nil fetch, "installer must fetch and verify source before review"
    assert_includes fetch["args"], ORIGIN
    assert_equal @sha, fetch["args"].last
    assert_includes fetch["args"], "--no-tags"
    build = events("go").find { |e| e["args"].first == "build" }
    refute_nil build
    refute_equal @caller, build["cwd"]
    assert_equal "off", build["gowork"]
    assert_includes build["args"], "-buildvcs=true"
    assert_includes build["args"], "-mod=readonly"
    assert_equal "./cmd/octostate", build["args"].last
    assert_includes diagnostic, @sha
    assert_includes diagnostic, "vcs.modified=false"
    assert_equal "#{File.join(@runner, "octostate-bin")}\n", File.read(@env.fetch("GITHUB_PATH"))
    assert_equal %w[validate plan apply], events("octostate").map { |e| e["args"][1] }
    assert_empty Dir.glob(File.join(@runner, "octostate-source.*")), "temporary source must be removed"
  end

  def test_real_source_installer_smoke
    sha = ENV["CONFIG_REVIEW_REAL_SOURCE_SMOKE_SHA"]
    skip "set CONFIG_REVIEW_REAL_SOURCE_SMOKE_SHA in CI to run the real-source smoke test" unless sha

    assert_match(/\A[0-9a-f]{40}\z/, sha)
    install_step = JOB.fetch("steps").find { |step| step["name"] == "Install Octostate" }
    bin_dir = File.join(@runner, "real-source-smoke-bin")
    env = {
      "PATH" => ENV.fetch("PATH"),
      "HOME" => ENV.fetch("HOME"),
      "GOCACHE" => File.join(@tmp, "go-build-cache"),
      "GOTOOLCHAIN" => "local",
      "RUNNER_TEMP" => @runner, "GITHUB_PATH" => @env.fetch("GITHUB_PATH"),
      "OCTOSTATE_VERSION" => sha, "OCTOSTATE_BIN_DIR" => bin_dir
    }
    %w[HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy].each do |key|
      env[key] = ENV[key] if ENV.key?(key)
    end

    out, err, status = Open3.capture3(
      env, "bash", "--noprofile", "--norc", "-c", install_step.fetch("run"),
      chdir: @caller, unsetenv_others: true
    )
    assert status.success?, "real installer failed (#{status.exitstatus}):\n#{out}#{err}"

    binary = File.join(bin_dir, "octostate")
    assert File.executable?(binary), "installer must produce an executable"
    metadata, metadata_status = Open3.capture2(
      env, "go", "version", "-m", binary, unsetenv_others: true
    )
    assert metadata_status.success?, "go version -m failed: #{metadata}"
    assert_includes metadata, "vcs.revision=#{sha}"
    assert_includes metadata, "vcs.modified=false"
    _, help_err, help_status = Open3.capture3(env, binary, "--help", unsetenv_others: true)
    assert help_status.success?, "installed Octostate could not run: #{help_err}"
    assert_equal "#{bin_dir}\n", File.read(@env.fetch("GITHUB_PATH"))
    assert_empty Dir.glob(File.join(@runner, "octostate-source.*")), "temporary source must be removed"
  end

  def test_rejects_invalid_revisions_before_fetch_or_build
    ["v1.2.0", "main", "latest", @sha[0, 12], "", @sha.upcase, " #{@sha}", "#{@sha}\n", "$(touch injected)", "#{@sha}x"].each do |revision|
      reset_run
      @inputs["octostate_version"] = revision
      refute run_review.success?, "accepted invalid revision #{revision.inspect}"
      assert_empty events, diagnostic
      assert_empty File.read(@env.fetch("GITHUB_PATH"))
      assert_includes diagnostic, "octostate_version"
      refute File.exist?(File.join(@caller, "injected"))
    end
  end

  def test_source_or_binary_identity_failures_stop_before_credentials
    [
      {"FAIL_GIT" => "fetch"}, {"FAIL_GIT" => "checkout"}, {"WRONG_HEAD" => "1"},
      {"FAIL_GO" => "build"}, {"FAIL_GO" => "version"}, {"MISSING_REVISION" => "1"},
      {"METADATA_REVISION" => "0" * 40}, {"DIRTY" => "true"}
    ].each do |failure|
      reset_run
      refute run_review(failure).success?, "accepted #{failure.inspect}"
      assert_empty events("octostate"), diagnostic
      assert_empty File.read(@env.fetch("GITHUB_PATH"))
      assert_empty Dir.glob(File.join(@runner, "octostate-source.*"))
    end
  end

  def test_moved_tag_cannot_replace_the_selected_commit
    git("-C", @source, "tag", "-f", "v1.0.0", @sha)
    @inputs["octostate_version"] = @old_sha
    assert run_review.success?, diagnostic
    state = JSON.parse(File.read(@env.fetch("BUILD_STATE")))
    assert_equal @old_sha, state.fetch("revision")
    assert_includes diagnostic, @old_sha
    refute events("go").any? { |e| e["args"].first == "install" }
  end

  def test_installation_and_validation_have_no_organization_credentials
    assert run_review.success?, diagnostic
    refute events("git").any? { |e| e["token"] }
    refute events("go").any? { |e| e["token"] }
    refute events("octostate").first.fetch("token")
  end

  def test_configuration_path_is_a_single_argument
    @inputs["config_dir"] = "config directory; touch injected"
    assert run_review.success?, diagnostic
    events("octostate").each do |event|
      args = event.fetch("args")
      assert_equal @inputs["config_dir"], args.fetch(args.index("--config-dir") + 1)
    end
    refute File.exist?(File.join(@caller, "injected"))
  end

  def test_live_commands_authenticate_without_token_arguments_or_persistence
    assert run_review("ASSERT_ENV_AUTH" => "1").success?, diagnostic
    calls = events("octostate")
    assert_equal ["config", "plan", "--config-dir", "config directory", "--expected-org", "acme"], calls[1].fetch("args")
    assert_equal ["config", "apply", "--config-dir", "config directory", "--check", "--expected-org", "acme"], calls[2].fetch("args")
    calls.drop(1).each { |call| assert call.fetch("correct_token") }
    refute_includes diagnostic, SECRET
    refute_includes File.read(@env.fetch("EVENTS")), SECRET
    %w[GITHUB_PATH GITHUB_ENV GITHUB_OUTPUT GITHUB_STEP_SUMMARY].each do |key|
      refute_includes File.read(@env.fetch(key)), SECRET
    end
  end

  def test_missing_credentials_stop_each_live_command
    @secret = ""
    JOB.fetch("steps").select { |s| ["Plan configuration", "Preflight configuration apply"].include?(s["name"]) }.each do |step|
      reset_run
      refute run_step(step).success?
      assert_includes diagnostic, "octostate_token is required"
      assert_empty events("octostate")
    end
  end

  def test_expected_organization_is_forwarded_to_both_live_commands
    @inputs["expected_org"] = "acme"
    assert run_review("ASSERT_ENV_AUTH" => "1").success?, diagnostic
    assert_equal ["config", "plan", "--config-dir", "config directory", "--expected-org", "acme"], events("octostate")[1].fetch("args")
    assert_equal ["config", "apply", "--config-dir", "config directory", "--check", "--expected-org", "acme"], events("octostate")[2].fetch("args")
  end

  def test_empty_expected_organization_stops_before_plan
    @inputs["expected_org"] = ""
    refute run_review.success?
    assert_equal ["validate"], events("octostate").map { |call| call["args"][1] }
    assert_includes diagnostic, "expected_org is required"
    refute_includes diagnostic, SECRET
  end

  def test_expected_org_is_required_without_a_binding_toggle
    inputs = WORKFLOW.fetch(true).fetch("workflow_call").fetch("inputs")
    assert_equal true, inputs.fetch("expected_org").fetch("required")
    refute inputs.key?("bind_expected_org")
  end

  def test_command_failures_preserve_output_and_stop_later_steps
    %w[validate plan apply].each_with_index do |command, index|
      reset_run
      status = run_review("FAIL_COMMAND" => command)
      assert_equal 23, status.exitstatus, diagnostic
      assert_equal %w[validate plan apply].take(index + 1), events("octostate").map { |call| call["args"][1] }
      result = @results.last
      assert_equal "{\"status\":\"ok\"}\n", result.fetch(:out)
      assert_equal "command diagnostic\n", result.fetch(:err)
    end
  end
end
