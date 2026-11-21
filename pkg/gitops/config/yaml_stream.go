package config

import (
	"fmt"
	"io"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/gitops/internal/sizeguard"
	"gopkg.in/yaml.v3"
)

// preflightDesiredYAML counts the canonical document without constructing a
// yaml.Node tree and rejects scalars yaml.v3 would copy past the output cap.
func preflightDesiredYAML(cfg OrganizationConfig, maxNodes, maxBytes int64) error {
	count := desiredYAMLNodeCounter{limit: maxNodes}
	count.add(12) // Document, root mapping, and five key/sequence pairs.
	if count.exceeded {
		return desiredStateYAMLNodeLimitError(maxNodes)
	}
	for range cfg.Members {
		count.add(5) // Record mapping and username/role key/value nodes.
		if count.exceeded {
			return desiredStateYAMLNodeLimitError(maxNodes)
		}
	}
	for _, invite := range cfg.Invites {
		count.add(desiredYAMLInviteNodeCount(invite))
		if count.exceeded {
			return desiredStateYAMLNodeLimitError(maxNodes)
		}
	}
	for _, repo := range cfg.Repositories {
		count.add(desiredYAMLRepositoryNodeCount(repo, cfg.Organization))
		if count.exceeded {
			return desiredStateYAMLNodeLimitError(maxNodes)
		}
	}
	for _, team := range cfg.Teams {
		count.add(desiredYAMLTeamNodeCount(team, cfg.Organization))
		if count.exceeded {
			return desiredStateYAMLNodeLimitError(maxNodes)
		}
	}
	if maxBytes < 0 || maxDesiredYAMLScalarLength(cfg) > maxBytes {
		return desiredStateSizeLimitError(&sizeguard.LimitExceededError{Limit: maxBytes})
	}
	return nil
}

type desiredYAMLNodeCounter struct {
	limit    int64
	nodes    int64
	exceeded bool
}

func (c *desiredYAMLNodeCounter) add(amount int64) {
	if c.exceeded || amount <= 0 {
		return
	}
	if c.limit < 0 || amount > c.limit-c.nodes {
		c.exceeded = true
		return
	}
	c.nodes += amount
}

func emittedRepositoryOwner(owner, organization string) bool {
	owner = strings.TrimSpace(owner)
	return owner != "" && !strings.EqualFold(owner, strings.TrimSpace(organization))
}

func desiredRepositoryStringNode(value OptionalString, fallback string) (*yaml.Node, bool) {
	if value.Present {
		if value.Null {
			return nullNode(), true
		}
		return stringNode(value.Value), true
	}
	if strings.TrimSpace(fallback) == "" {
		return nil, false
	}
	return stringNode(fallback), true
}

func desiredRepositoryBoolNode(value OptionalBool, effective bool) (*yaml.Node, bool) {
	if value.Present {
		if value.Null {
			return nullNode(), true
		}
		return boolNode(effective), true
	}
	if !effective {
		return nil, false
	}
	return boolNode(true), true
}

func desiredRepositoryStringIsEmitted(value OptionalString, fallback string) bool {
	return value.Present || strings.TrimSpace(fallback) != ""
}

func desiredRepositoryBoolIsEmitted(value OptionalBool, effective bool) bool {
	return value.Present || effective
}

func desiredYAMLTemplateFieldCount(template TemplateSpec) int64 {
	var count int64
	if strings.TrimSpace(template.Owner) != "" {
		count += 2
	}
	if strings.TrimSpace(template.Name) != "" {
		count += 2
	}
	if template.IncludeAllBranches {
		count += 2
	}
	return count
}

func maxDesiredYAMLScalarLength(cfg OrganizationConfig) int64 {
	var maximum int64
	consider := func(value string) {
		if int64(len(value)) > maximum {
			maximum = int64(len(value))
		}
	}
	consider(cfg.Organization)
	for _, member := range cfg.Members {
		consider(member.Username)
		consider(member.Role)
	}
	for _, invite := range cfg.Invites {
		if invite.Username.Present && !invite.Username.Null {
			consider(invite.Username.Value)
		}
		if invite.Email.Present && !invite.Email.Null {
			consider(invite.Email.Value)
		}
		consider(invite.Role)
		for _, slug := range invite.TeamSlugs {
			consider(slug)
		}
	}
	for _, repo := range cfg.Repositories {
		if emittedRepositoryOwner(repo.Owner, cfg.Organization) {
			consider(strings.TrimSpace(repo.Owner))
		}
		consider(repo.Name)
		if strings.TrimSpace(repo.Template.Owner) != "" {
			consider(strings.TrimSpace(repo.Template.Owner))
		}
		if strings.TrimSpace(repo.Template.Name) != "" {
			consider(strings.TrimSpace(repo.Template.Name))
		}
		consider(repo.Visibility)
		if option := repo.DescriptionOption(); option.Present {
			if !option.Null {
				consider(option.Value)
			}
		} else if strings.TrimSpace(repo.Description) != "" {
			consider(repo.Description)
		}
		if option := repo.HomepageOption(); option.Present {
			if !option.Null {
				consider(option.Value)
			}
		} else if strings.TrimSpace(repo.Homepage) != "" {
			consider(repo.Homepage)
		}
		for _, topic := range repo.Topics {
			consider(strings.TrimSpace(topic))
		}
	}
	for _, team := range cfg.Teams {
		consider(team.Slug)
		consider(team.Name)
		consider(team.Description)
		consider(team.Privacy)
		consider(team.ParentSlug)
		for _, member := range team.Members {
			consider(member.Username)
			consider(member.Role)
		}
		for _, repo := range team.Repositories {
			if emittedRepositoryOwner(repo.Owner, cfg.Organization) {
				consider(strings.TrimSpace(repo.Owner))
			}
			consider(repo.Name)
			consider(repo.Permission)
		}
	}
	return maximum
}

type desiredYAMLWriter struct {
	writer         io.Writer
	maxBytes       int64
	written        int64
	limitExceeded  bool
	maxSourceUnits int64
	sourceUnits    int64
	sourceExceeded bool
}

func (w *desiredYAMLWriter) Write(p []byte) (int, error) {
	if w.sourceUnits == 0 {
		w.sourceUnits = 2
		if w.sourceUnits > w.maxSourceUnits {
			w.sourceExceeded = true
		}
	}
	remaining := w.maxBytes - w.written
	if remaining < 0 {
		remaining = 0
	}
	if int64(len(p)) > remaining {
		w.limitExceeded = true
		if remaining == 0 {
			return 0, &sizeguard.LimitExceededError{Limit: w.maxBytes}
		}
		n, err := w.writeAndAccount(p[:int(remaining)])
		if err != nil {
			return n, err
		}
		return n, &sizeguard.LimitExceededError{Limit: w.maxBytes}
	}
	return w.writeAndAccount(p)
}

func (w *desiredYAMLWriter) writeAndAccount(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.written += int64(n)
		for _, character := range p[:n] {
			weight := desiredYAMLSourceMarkerWeight(character)
			if weight == 0 {
				continue
			}
			if int64(weight) > w.maxSourceUnits-w.sourceUnits {
				w.sourceExceeded = true
				continue
			}
			w.sourceUnits += int64(weight)
		}
	}
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func (w *desiredYAMLWriter) sourceError() error {
	if !w.sourceExceeded {
		return nil
	}
	return fmt.Errorf("desired-state YAML exceeds the supported source complexity limit of %s", desiredStateYAMLSourceUnitsDisplay(w.maxSourceUnits))
}

type yamlIndentWriter struct {
	writer    io.Writer
	indent    []byte
	lineStart bool
	pending   [2]byte
	pendingN  int
}

func (w *yamlIndentWriter) Write(p []byte) (int, error) {
	consumed := 0
	if w.pendingN > 0 {
		var prefix [3]byte
		pendingN := w.pendingN
		copy(prefix[:], w.pending[:pendingN])
		take := min(len(p), len(prefix)-pendingN)
		copy(prefix[pendingN:], p[:take])
		lineBreak, incomplete := yamlLineBreakLength(prefix[:pendingN+take])
		if incomplete {
			copy(w.pending[pendingN:], p[:take])
			w.pendingN += take
			return take, nil
		}
		if lineBreak > 0 {
			w.pendingN = 0
			n, err := w.writeLineBreak(prefix[:lineBreak])
			if n > pendingN {
				consumed += n - pendingN
			}
			if err != nil {
				return consumed, err
			}
			p = p[lineBreak-pendingN:]
		} else {
			w.pendingN = 0
			if _, err := w.writeText(w.pending[:pendingN]); err != nil {
				return consumed, err
			}
		}
	}

	start := 0
	for i := 0; i < len(p); {
		lineBreak, incomplete := yamlLineBreakLength(p[i:])
		if incomplete {
			n, err := w.writeText(p[start:i])
			consumed += n
			if err != nil {
				return consumed, err
			}
			w.pendingN = copy(w.pending[:], p[i:])
			return consumed + len(p[i:]), nil
		}
		if lineBreak == 0 {
			i++
			continue
		}
		n, err := w.writeText(p[start:i])
		consumed += n
		if err != nil {
			return consumed, err
		}
		n, err = w.writeLineBreak(p[i : i+lineBreak])
		consumed += n
		if err != nil {
			return consumed, err
		}
		i += lineBreak
		start = i
	}
	n, err := w.writeText(p[start:])
	return consumed + n, err
}

func (w *yamlIndentWriter) writeLineBreak(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n == len(p) && err == nil {
		w.lineStart = true
	}
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func (w *yamlIndentWriter) writeText(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.lineStart && len(w.indent) > 0 {
		if err := writeAll(w.writer, w.indent); err != nil {
			return 0, err
		}
	}
	n, err := w.writer.Write(p)
	if n > 0 {
		w.lineStart = false
	}
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func yamlLineBreakLength(p []byte) (int, bool) {
	if len(p) == 0 {
		return 0, false
	}
	switch p[0] {
	case '\r', '\n':
		return 1, false
	case 0xc2:
		if len(p) == 1 {
			return 0, true
		}
		if p[1] == 0x85 {
			return 2, false
		}
	case 0xe2:
		if len(p) == 1 {
			return 0, true
		}
		if p[1] != 0x80 {
			return 0, false
		}
		if len(p) == 2 {
			return 0, true
		}
		if p[2] == 0xa8 || p[2] == 0xa9 {
			return 3, false
		}
	}
	return 0, false
}

func writeAll(writer io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := writer.Write(p)
		p = p[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func emitYAMLFragment(writer io.Writer, indent int, value *yaml.Node) error {
	output := writer
	if indent > 0 {
		output = &yamlIndentWriter{writer: writer, indent: []byte(strings.Repeat(" ", indent)), lineStart: true}
	}
	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	encodeErr := encoder.Encode(value)
	closeErr := encoder.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func emitMapEntry(writer io.Writer, indent int, key string, value *yaml.Node) error {
	item := mapNode()
	appendMapField(item, key, value)
	return emitYAMLFragment(writer, indent, item)
}

func emitSequenceItem(writer io.Writer, indent int, value *yaml.Node) error {
	return emitYAMLFragment(writer, indent, sequenceNode([]*yaml.Node{value}))
}

func writeIndentedString(writer io.Writer, indent int, value string) error {
	if indent == 0 {
		_, err := io.WriteString(writer, value)
		return err
	}
	return writeAll(&yamlIndentWriter{writer: writer, indent: []byte(strings.Repeat(" ", indent)), lineStart: true}, []byte(value))
}

func writeDesiredYAML(cfg OrganizationConfig, writer io.Writer) error {
	if err := emitMapEntry(writer, 0, "organization", stringNode(cfg.Organization)); err != nil {
		return err
	}
	if err := emitRootSequence(writer, "members", len(cfg.Members)); err != nil {
		return err
	}
	if err := emitDesiredYAMLSequenceChunks(writer, len(cfg.Members),
		func(int) int64 { return 5 },
		func(start, end int) *yaml.Node { return encodeMembers(cfg.Members[start:end]) },
		nil,
	); err != nil {
		return err
	}
	if err := writeInvitesYAML(writer, cfg.Invites); err != nil {
		return err
	}
	if err := writeRepositoriesYAML(writer, cfg.Repositories, cfg.Organization); err != nil {
		return err
	}
	return writeTeamsYAML(writer, cfg.Teams, cfg.Organization)
}

const maxDesiredYAMLFragmentNodes int64 = 2_048

func emitDesiredYAMLSequenceChunks(writer io.Writer, length int, nodeCount func(int) int64, encode func(int, int) *yaml.Node, emitOversized func(int) error) error {
	return emitDesiredYAMLSequenceChunksAtIndent(writer, 2, length, nodeCount, encode, emitOversized)
}

func emitDesiredYAMLSequenceChunksAtIndent(writer io.Writer, indent, length int, nodeCount func(int) int64, encode func(int, int) *yaml.Node, emitOversized func(int) error) error {
	for start := 0; start < length; {
		end := start
		chunkNodes := int64(1) // The sequence node.
		for end < length {
			rowNodes := nodeCount(end)
			if rowNodes+1 > maxDesiredYAMLFragmentNodes {
				if end != start {
					break
				}
				if emitOversized == nil {
					return fmt.Errorf("YAML fragment node estimate exceeds the bounded chunk size")
				}
				if err := emitOversized(start); err != nil {
					return err
				}
				start++
				end = start
				break
			}
			if chunkNodes+rowNodes > maxDesiredYAMLFragmentNodes {
				break
			}
			chunkNodes += rowNodes
			end++
		}
		if end == start {
			continue
		}
		if err := emitYAMLFragment(writer, indent, encode(start, end)); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func emitRootSequence(writer io.Writer, key string, length int) error {
	if length == 0 {
		return emitMapEntry(writer, 0, key, sequenceNode([]*yaml.Node{}))
	}
	return writeIndentedString(writer, 0, key+":\n")
}

func writeInvitesYAML(writer io.Writer, invites []InviteSpec) error {
	if err := emitRootSequence(writer, "invites", len(invites)); err != nil {
		return err
	}
	return emitDesiredYAMLSequenceChunks(writer, len(invites),
		func(index int) int64 { return desiredYAMLInviteNodeCount(invites[index]) },
		func(start, end int) *yaml.Node { return encodeInvites(invites[start:end]) },
		func(index int) error { return writeInviteYAML(writer, invites[index]) },
	)
}

func desiredYAMLInviteNodeCount(invite InviteSpec) int64 {
	count := int64(3) // Record mapping and role key/value.
	if invite.Username.Present {
		count += 2
	}
	if invite.Email.Present {
		count += 2
	}
	if invite.UserID.Present {
		count += 2
	}
	if len(invite.TeamSlugs) > 0 {
		count += 2 + int64(len(invite.TeamSlugs))
	}
	return count
}

func writeInviteYAML(writer io.Writer, invite InviteSpec) error {
	item := mapNode()
	if invite.Username.Present {
		appendMapField(item, "username", optionalStringNode(invite.Username))
	}
	if invite.Email.Present {
		appendMapField(item, "email", optionalStringNode(invite.Email))
	}
	if invite.UserID.Present {
		appendMapField(item, "user_id", optionalInt64Node(invite.UserID))
	}
	appendMapField(item, "role", stringNode(invite.Role))
	if err := emitSequenceItem(writer, 2, item); err != nil {
		return err
	}
	if len(invite.TeamSlugs) == 0 {
		return nil
	}
	if err := writeIndentedString(writer, 4, "team_slugs:\n"); err != nil {
		return err
	}
	return emitDesiredYAMLSequenceChunksAtIndent(writer, 6, len(invite.TeamSlugs),
		func(int) int64 { return 1 },
		func(start, end int) *yaml.Node { return encodeStringSequence(invite.TeamSlugs[start:end]) },
		nil,
	)
}

func writeRepositoriesYAML(writer io.Writer, repositories []RepositorySpec, organization string) error {
	if err := emitRootSequence(writer, "repositories", len(repositories)); err != nil {
		return err
	}
	return emitDesiredYAMLSequenceChunks(writer, len(repositories),
		func(index int) int64 { return desiredYAMLRepositoryNodeCount(repositories[index], organization) },
		func(start, end int) *yaml.Node { return encodeRepositories(repositories[start:end], organization) },
		func(index int) error { return writeRepositoryYAML(writer, repositories[index], organization) },
	)
}

func desiredYAMLRepositoryNodeCount(repo RepositorySpec, organization string) int64 {
	count := int64(5) // Record mapping and name/visibility key/value nodes.
	if emittedRepositoryOwner(repo.Owner, organization) {
		count += 2
	}
	if templateFields := desiredYAMLTemplateFieldCount(repo.Template); templateFields > 0 {
		count += 2 + templateFields
	}
	if desiredRepositoryStringIsEmitted(repo.DescriptionOption(), repo.Description) {
		count += 2
	}
	if desiredRepositoryStringIsEmitted(repo.HomepageOption(), repo.Homepage) {
		count += 2
	}
	if len(repo.Topics) > 0 {
		count += 2 + int64(len(repo.Topics))
	}
	if desiredRepositoryBoolIsEmitted(repo.AllowForkingOption(), repo.AllowForking) {
		count += 2
	}
	if desiredRepositoryBoolIsEmitted(repo.ArchivedOption(), repo.Archived) {
		count += 2
	}
	if desiredRepositoryBoolIsEmitted(repo.IsTemplateOption(), repo.IsTemplate) {
		count += 2
	}
	return count
}

func writeRepositoryYAML(writer io.Writer, repo RepositorySpec, organization string) error {
	item := mapNode()
	if emittedRepositoryOwner(repo.Owner, organization) {
		appendMapField(item, "owner", stringNode(strings.TrimSpace(repo.Owner)))
	}
	appendMapField(item, "name", stringNode(repo.Name))
	if template := encodeTemplate(repo.Template); len(template.Content) > 0 {
		appendMapField(item, "template", template)
	}
	appendMapField(item, "visibility", stringNode(repo.Visibility))
	if value, ok := desiredRepositoryStringNode(repo.DescriptionOption(), repo.Description); ok {
		appendMapField(item, "description", value)
	}
	if value, ok := desiredRepositoryStringNode(repo.HomepageOption(), repo.Homepage); ok {
		appendMapField(item, "homepage", value)
	}
	if len(repo.Topics) == 0 {
		appendDesiredRepositoryBooleans(item, repo)
		return emitSequenceItem(writer, 2, item)
	}
	if err := emitSequenceItem(writer, 2, item); err != nil {
		return err
	}
	if err := writeIndentedString(writer, 4, "topics:\n"); err != nil {
		return err
	}
	if err := emitDesiredYAMLSequenceChunksAtIndent(writer, 6, len(repo.Topics),
		func(int) int64 { return 1 },
		func(start, end int) *yaml.Node { return encodeStringSequence(repo.Topics[start:end]) },
		nil,
	); err != nil {
		return err
	}
	if value, ok := desiredRepositoryBoolNode(repo.AllowForkingOption(), repo.AllowForking); ok {
		if err := emitMapEntry(writer, 4, "allow_forking", value); err != nil {
			return err
		}
	}
	if value, ok := desiredRepositoryBoolNode(repo.ArchivedOption(), repo.Archived); ok {
		if err := emitMapEntry(writer, 4, "archived", value); err != nil {
			return err
		}
	}
	if value, ok := desiredRepositoryBoolNode(repo.IsTemplateOption(), repo.IsTemplate); ok {
		if err := emitMapEntry(writer, 4, "is_template", value); err != nil {
			return err
		}
	}
	return nil
}

func appendDesiredRepositoryBooleans(item *yaml.Node, repo RepositorySpec) {
	if value, ok := desiredRepositoryBoolNode(repo.AllowForkingOption(), repo.AllowForking); ok {
		appendMapField(item, "allow_forking", value)
	}
	if value, ok := desiredRepositoryBoolNode(repo.ArchivedOption(), repo.Archived); ok {
		appendMapField(item, "archived", value)
	}
	if value, ok := desiredRepositoryBoolNode(repo.IsTemplateOption(), repo.IsTemplate); ok {
		appendMapField(item, "is_template", value)
	}
}

func writeTeamsYAML(writer io.Writer, teams []TeamSpec, organization string) error {
	if err := emitRootSequence(writer, "teams", len(teams)); err != nil {
		return err
	}
	return emitDesiredYAMLSequenceChunks(writer, len(teams),
		func(index int) int64 { return desiredYAMLTeamNodeCount(teams[index], organization) },
		func(start, end int) *yaml.Node { return encodeTeams(teams[start:end], organization) },
		func(index int) error { return writeTeamYAML(writer, teams[index], organization) },
	)
}

func desiredYAMLTeamNodeCount(team TeamSpec, organization string) int64 {
	count := int64(7) // Record mapping and slug/name/privacy key/value nodes.
	if team.Description != "" {
		count += 2
	}
	if team.ParentSlug != "" {
		count += 2
	}
	if len(team.Members) > 0 {
		count += 2 + 5*int64(len(team.Members))
	}
	if len(team.Repositories) > 0 {
		count += 2
		for _, repo := range team.Repositories {
			count += 5
			if emittedRepositoryOwner(repo.Owner, organization) {
				count += 2
			}
		}
	}
	return count
}

func writeTeamYAML(writer io.Writer, team TeamSpec, organization string) error {
	item := mapNode()
	appendMapField(item, "slug", stringNode(team.Slug))
	appendMapField(item, "name", stringNode(team.Name))
	if team.Description != "" {
		appendMapField(item, "description", stringNode(team.Description))
	}
	appendMapField(item, "privacy", stringNode(team.Privacy))
	if team.ParentSlug != "" {
		appendMapField(item, "parent_slug", stringNode(team.ParentSlug))
	}
	if err := emitSequenceItem(writer, 2, item); err != nil {
		return err
	}
	if len(team.Members) > 0 {
		if err := writeIndentedString(writer, 4, "members:\n"); err != nil {
			return err
		}
		if err := emitDesiredYAMLSequenceChunksAtIndent(writer, 6, len(team.Members),
			func(int) int64 { return 5 },
			func(start, end int) *yaml.Node { return encodeTeamMembers(team.Members[start:end]) },
			nil,
		); err != nil {
			return err
		}
	}
	if len(team.Repositories) > 0 {
		if err := writeIndentedString(writer, 4, "repositories:\n"); err != nil {
			return err
		}
		if err := emitDesiredYAMLSequenceChunksAtIndent(writer, 6, len(team.Repositories),
			func(index int) int64 {
				if emittedRepositoryOwner(team.Repositories[index].Owner, organization) {
					return 7
				}
				return 5
			},
			func(start, end int) *yaml.Node {
				return encodeTeamRepositories(team.Repositories[start:end], organization)
			},
			nil,
		); err != nil {
			return err
		}
	}
	return nil
}

func encodeStringSequence(values []string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(values))
	for _, value := range values {
		items = append(items, stringNode(value))
	}
	return sequenceNode(items)
}

func optionalStringNode(value OptionalString) *yaml.Node {
	if value.Null {
		return nullNode()
	}
	return stringNode(value.Value)
}

func optionalInt64Node(value OptionalInt64) *yaml.Node {
	if value.Null {
		return nullNode()
	}
	return int64Node(value.Value)
}
