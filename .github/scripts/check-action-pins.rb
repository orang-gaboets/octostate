require "yaml"

module ActionPinChecker
  GITHUB_ACTION_REF = /\A[^@\s\/]+\/[^@\s]+@[0-9a-f]{40}\z/.freeze
  DOCKER_ACTION_REF = /\Adocker:\/\/[^@\s]+@sha256:[0-9a-f]{64}\z/.freeze

  def self.default_workflows
    Dir.glob(".github/workflows/*.{yml,yaml}").sort
  end

  def self.uses_values(workflow)
    jobs = workflow.is_a?(Hash) ? workflow["jobs"] : nil
    return [] unless jobs.is_a?(Hash)

    values = []
    jobs.each_value do |job|
      next unless job.is_a?(Hash)

      values << job["uses"] if job.key?("uses")
      steps = job["steps"]
      if steps.is_a?(Array)
        steps.each do |step|
          values << step["uses"] if step.is_a?(Hash) && step.key?("uses")
        end
      end
    end
    values
  end

  def self.display_ref(value)
    return "<empty>" if value == ""
    return value if value.is_a?(String)

    "<#{value.class}>"
  end

  def self.invalid_ref_reason(value)
    return "uses value must be a non-empty string" unless value.is_a?(String) && !value.empty?
    return "local action path must name a directory after ./" if value == "./"
    return nil if value.start_with?("./")
    return nil if value.match?(DOCKER_ACTION_REF)
    return "docker action must use an immutable sha256 digest" if value.start_with?("docker://")
    return nil if value.match?(GITHUB_ACTION_REF)

    "external action/workflow must use one @ followed by a 40-character lowercase commit SHA"
  end

  def self.violations(paths)
    paths = default_workflows if paths.empty?
    errors = []

    paths.sort.each do |path|
      begin
        document = YAML.safe_load(File.read(path), aliases: true)
      rescue Psych::Exception => error
        errors << [path, "<invalid YAML>", "invalid YAML: #{error.message.lines.first.to_s.strip}"]
        next
      rescue SystemCallError
        errors << [path, "<unreadable workflow>", "unable to read workflow file"]
        next
      end

      uses_values(document).each do |value|
        reason = invalid_ref_reason(value)
        errors << [path, display_ref(value), reason] if reason
      end
    end

    errors.sort_by { |path, reference, reason| [path, reference, reason] }
  end

  def self.run(paths = ARGV)
    errors = violations(paths)
    errors.each do |path, reference, reason|
      warn("#{path}: #{reason}: #{reference}")
    end
    errors.empty? ? 0 : 1
  end
end

exit ActionPinChecker.run if $PROGRAM_NAME == __FILE__
