require "fileutils"
require "minitest/autorun"
require "open3"
require "rbconfig"
require "tmpdir"

ROOT = File.expand_path("../..", __dir__)
CHECKER = File.join(__dir__, "check-action-pins.rb")
SHA = "0123456789abcdef0123456789abcdef01234567"
DOCKER_DIGEST = "a" * 64

class CheckActionPinsTest < Minitest::Test
  def setup
    @tmpdir = Dir.mktmpdir("check-action-pins-")
  end

  def teardown
    FileUtils.remove_entry(@tmpdir)
  end

  def write_workflow(name, source)
    path = File.join(@tmpdir, name)
    File.write(path, source)
    path
  end

  def run_checker(*paths)
    Open3.capture3(RbConfig.ruby, CHECKER, *paths, chdir: ROOT)
  end

  def test_accepts_pinned_step_and_job_refs_and_local_actions
    path = write_workflow("valid.yml", <<~YAML)
      jobs:
        reusable:
          uses: owner/reusable/.github/workflows/ci.yml@#{SHA}
        build:
          steps:
            - uses: owner/action/subpath@#{SHA}
            - uses: ./actions/local
    YAML

    stdout, stderr, status = run_checker(path)

    assert_predicate status, :success?, "#{stdout}#{stderr}"
    assert_empty stderr
  end

  def test_ignores_action_inputs_named_uses
    path = write_workflow("uses-input.yml", <<~YAML)
      jobs:
        build:
          steps:
            - uses: ./actions/wrapper
              with:
                uses: main
    YAML

    stdout, stderr, status = run_checker(path)

    assert_predicate status, :success?, "#{stdout}#{stderr}"
    assert_empty stderr
  end

  def test_rejects_mutable_external_github_refs_with_path_and_ref
    path = write_workflow("mutable.yml", <<~YAML)
      jobs:
        build:
          steps:
            - uses: actions/checkout@v7
    YAML

    _stdout, stderr, status = run_checker(path)

    refute_predicate status, :success?
    assert_includes stderr, "mutable.yml"
    assert_includes stderr, "actions/checkout@v7"
  end

  def test_rejects_mutable_reusable_workflow_refs
    path = write_workflow("mutable-workflow.yml", <<~YAML)
      jobs:
        reusable:
          uses: owner/repo/.github/workflows/build.yml@main
    YAML

    _stdout, stderr, status = run_checker(path)

    refute_predicate status, :success?
    assert_includes stderr, "mutable-workflow.yml"
    assert_includes stderr, "owner/repo/.github/workflows/build.yml@main"
  end

  def test_docker_actions_require_an_immutable_digest
    path = write_workflow("docker.yml", <<~YAML)
      jobs:
        build:
          steps:
            - uses: docker://alpine:3.20
            - uses: docker://ghcr.io/example/action:1.0@sha256:#{DOCKER_DIGEST}
    YAML

    _stdout, stderr, status = run_checker(path)

    refute_predicate status, :success?
    assert_includes stderr, "docker://alpine:3.20"
    refute_includes stderr, "ghcr.io/example/action:1.0@sha256:#{DOCKER_DIGEST}"
  end

  def test_rejects_empty_non_string_and_multiple_at_refs
    path = write_workflow("invalid-uses.yml", <<~YAML)
      jobs:
        build:
          steps:
            - uses: ""
            - uses: 42
            - uses: owner/action@#{SHA}@extra
    YAML

    _stdout, stderr, status = run_checker(path)

    refute_predicate status, :success?
    assert_includes stderr, "<empty>"
    assert_includes stderr, "<Integer>"
    assert_includes stderr, "owner/action@#{SHA}@extra"
  end

  def test_fails_clearly_on_malformed_yaml
    path = write_workflow("malformed.yml", "jobs:\n  broken: [\n")

    _stdout, stderr, status = run_checker(path)

    refute_predicate status, :success?
    assert_includes stderr, "malformed.yml"
    assert_includes stderr, "invalid YAML"
  end

  def test_sorts_diagnostics_independently_of_input_order
    zeta = write_workflow("zeta.yml", "jobs:\n  build:\n    steps:\n      - uses: zeta/action@v1\n")
    alpha = write_workflow("alpha.yml", "jobs:\n  build:\n    steps:\n      - uses: alpha/action@v1\n")

    _stdout, stderr, status = run_checker(zeta, alpha)

    refute_predicate status, :success?
    alpha_index = stderr.index("alpha.yml")
    zeta_index = stderr.index("zeta.yml")
    refute_nil alpha_index
    refute_nil zeta_index
    assert_operator alpha_index, :<, zeta_index
  end
end
