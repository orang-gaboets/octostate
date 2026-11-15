package snapshot

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/orang-gaboets/octostate/pkg/gitops/internal/sizeguard"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

type snapshotJSONWriteContainer struct {
	kind              byte
	first             bool
	expectObjectValue bool
	entries           int64
}

type snapshotJSONEmitter struct {
	writer        io.Writer
	limits        snapshotJSONLimits
	maxBytes      int64
	written       int64
	tokens        int64
	arrayElements int64
	containers    []snapshotJSONWriteContainer
	structuralErr error
	stringScratch [32 * 1024]byte
}

type snapshotEncodingError struct {
	err error
}

func (e *snapshotEncodingError) Error() string { return e.err.Error() }
func (e *snapshotEncodingError) Unwrap() error { return e.err }

type snapshotOutputWriteError struct {
	err error
}

func (e *snapshotOutputWriteError) Error() string { return e.err.Error() }
func (e *snapshotOutputWriteError) Unwrap() error { return e.err }

func actualSnapshotHasScalarAboveLimit(snapshot ActualSnapshot, maxBytes int64) bool {
	if snapshotStringAboveLimit(snapshot.Organization, maxBytes) {
		return true
	}
	for username, userID := range snapshot.ResolvedInviteUserIDsByUsername {
		if userID > 0 && resolvedInviteUsernameAboveLimit(username, maxBytes) {
			return true
		}
	}
	return membersHaveOversizedScalar(snapshot.Members, maxBytes) ||
		invitationsHaveOversizedScalar(snapshot.PendingInvitations, maxBytes) ||
		repositoriesHaveOversizedScalar(snapshot.Repositories, maxBytes) ||
		teamsHaveOversizedScalar(snapshot.Teams, maxBytes) ||
		teamMembersHaveOversizedScalar(snapshot.TeamMembers, maxBytes) ||
		teamPermissionsHaveOversizedScalar(snapshot.TeamRepositoryPermissions, maxBytes)
}

func snapshotStringAboveLimit(value string, maxBytes int64) bool {
	return int64(len(value)) > maxBytes
}

func resolvedInviteUsernameAboveLimit(username string, maxBytes int64) bool {
	username = strings.TrimSpace(username)
	var normalizedBytes int64
	for len(username) > 0 {
		value, size := utf8.DecodeRuneInString(username)
		normalizedBytes += int64(utf8.RuneLen(unicode.ToLower(value)))
		if normalizedBytes > maxBytes {
			return true
		}
		username = username[size:]
	}
	return false
}

func membersHaveOversizedScalar(values []state.OrganizationMember, maxBytes int64) bool {
	for _, value := range values {
		if snapshotStringAboveLimit(value.Username, maxBytes) || snapshotStringAboveLimit(value.Role, maxBytes) || snapshotStringAboveLimit(value.Name, maxBytes) || snapshotStringAboveLimit(value.Email, maxBytes) {
			return true
		}
	}
	return false
}

func invitationsHaveOversizedScalar(values []state.PendingInvitation, maxBytes int64) bool {
	for _, value := range values {
		if snapshotStringAboveLimit(value.Username, maxBytes) || snapshotStringAboveLimit(value.Email, maxBytes) || snapshotStringAboveLimit(value.Role, maxBytes) {
			return true
		}
		for _, slug := range value.TeamSlugs {
			if snapshotStringAboveLimit(slug, maxBytes) {
				return true
			}
		}
	}
	return false
}

func repositoriesHaveOversizedScalar(values []state.Repository, maxBytes int64) bool {
	for _, value := range values {
		if snapshotStringAboveLimit(value.Owner, maxBytes) || snapshotStringAboveLimit(value.Name, maxBytes) || snapshotStringAboveLimit(value.Visibility, maxBytes) || snapshotStringAboveLimit(value.Description, maxBytes) || snapshotStringAboveLimit(value.Homepage, maxBytes) {
			return true
		}
		for _, topic := range value.Topics {
			if snapshotStringAboveLimit(topic, maxBytes) {
				return true
			}
		}
	}
	return false
}

func teamsHaveOversizedScalar(values []state.Team, maxBytes int64) bool {
	for _, value := range values {
		if snapshotStringAboveLimit(value.Slug, maxBytes) || snapshotStringAboveLimit(value.Name, maxBytes) || snapshotStringAboveLimit(value.Description, maxBytes) || snapshotStringAboveLimit(value.Privacy, maxBytes) || snapshotStringAboveLimit(value.ParentSlug, maxBytes) {
			return true
		}
	}
	return false
}

func teamMembersHaveOversizedScalar(values []state.TeamMember, maxBytes int64) bool {
	for _, value := range values {
		if snapshotStringAboveLimit(value.TeamSlug, maxBytes) || snapshotStringAboveLimit(value.Username, maxBytes) || snapshotStringAboveLimit(value.Role, maxBytes) {
			return true
		}
	}
	return false
}

func teamPermissionsHaveOversizedScalar(values []state.TeamRepositoryPermission, maxBytes int64) bool {
	for _, value := range values {
		if snapshotStringAboveLimit(value.TeamSlug, maxBytes) || snapshotStringAboveLimit(value.Owner, maxBytes) || snapshotStringAboveLimit(value.Name, maxBytes) || snapshotStringAboveLimit(value.Permission, maxBytes) {
			return true
		}
	}
	return false
}

func writeActualSnapshotJSON(writer io.Writer, snapshot ActualSnapshot, limits snapshotJSONLimits, maxBytes int64) error {
	buffered := bufio.NewWriterSize(writer, 64<<10)
	emitter := snapshotJSONEmitter{writer: buffered, limits: limits, maxBytes: maxBytes}
	if limits.tokens < 0 || limits.arrayElements < 0 || limits.objectEntriesPerObject < 0 {
		emitter.structuralErr = fmt.Errorf("JSON complexity limits must not be negative")
	}
	if err := emitter.actualSnapshot(snapshot); err != nil {
		return classifySnapshotJSONError(err)
	}
	if err := emitter.write([]byte{'\n'}); err != nil {
		return classifySnapshotJSONError(err)
	}
	if err := buffered.Flush(); err != nil {
		return &snapshotOutputWriteError{err: err}
	}
	if emitter.structuralErr != nil {
		return &snapshotEncodingError{err: emitter.structuralErr}
	}
	return nil
}

func classifySnapshotJSONError(err error) error {
	if errors.Is(err, sizeguard.ErrLimitExceeded) {
		return &snapshotEncodingError{err: actualSnapshotSizeLimitError(err)}
	}
	var outputErr *snapshotOutputWriteError
	if errors.As(err, &outputErr) {
		return outputErr
	}
	return &snapshotEncodingError{err: err}
}

func (e *snapshotJSONEmitter) actualSnapshot(snapshot ActualSnapshot) error {
	if err := e.beginObject(); err != nil {
		return err
	}
	if err := e.key("pulled_at"); err != nil {
		return err
	}
	pulledAt, err := snapshot.PulledAt.MarshalJSON()
	if err != nil {
		return err
	}
	if err := e.rawValue(pulledAt); err != nil {
		return err
	}
	if err := e.stringField("organization", snapshot.Organization); err != nil {
		return err
	}
	if err := e.key("resolved_invite_user_ids_by_username"); err != nil {
		return err
	}
	if err := e.int64Map(snapshot.ResolvedInviteUserIDsByUsername); err != nil {
		return err
	}
	if err := e.key("members"); err != nil {
		return err
	}
	if err := e.memberArray(snapshot.Members); err != nil {
		return err
	}
	if err := e.key("pending_invitations"); err != nil {
		return err
	}
	if err := e.invitationArray(snapshot.PendingInvitations); err != nil {
		return err
	}
	if err := e.key("repositories"); err != nil {
		return err
	}
	if err := e.repositoryArray(snapshot.Repositories); err != nil {
		return err
	}
	if err := e.key("teams"); err != nil {
		return err
	}
	if err := e.teamArray(snapshot.Teams); err != nil {
		return err
	}
	if err := e.key("team_members"); err != nil {
		return err
	}
	if err := e.teamMemberArray(snapshot.TeamMembers); err != nil {
		return err
	}
	if err := e.key("team_repo_permissions"); err != nil {
		return err
	}
	if err := e.teamRepositoryPermissionArray(snapshot.TeamRepositoryPermissions); err != nil {
		return err
	}
	if err := e.endObject(); err != nil {
		return err
	}
	return nil
}

func (e *snapshotJSONEmitter) write(p []byte) error {
	remaining := e.maxBytes - e.written
	if remaining < 0 {
		remaining = 0
	}
	if int64(len(p)) > remaining {
		if remaining > 0 {
			n, err := e.writer.Write(p[:int(remaining)])
			e.written += int64(n)
			if err != nil {
				return &snapshotOutputWriteError{err: err}
			}
			if n != int(remaining) {
				return &snapshotOutputWriteError{err: io.ErrShortWrite}
			}
		}
		return &sizeguard.LimitExceededError{Limit: e.maxBytes}
	}
	n, err := e.writer.Write(p)
	e.written += int64(n)
	if err != nil {
		return &snapshotOutputWriteError{err: err}
	}
	if n != len(p) {
		return &snapshotOutputWriteError{err: io.ErrShortWrite}
	}
	return nil
}

func (e *snapshotJSONEmitter) token() {
	e.tokens++
	if e.structuralErr == nil && e.tokens > e.limits.tokens {
		e.structuralErr = fmt.Errorf("actual-state snapshot exceeds the supported JSON token limit of %s", snapshotJSONTokenLimitDisplay(e.limits.tokens))
	}
}

func (e *snapshotJSONEmitter) beforeValue() error {
	if len(e.containers) == 0 {
		return nil
	}
	parent := &e.containers[len(e.containers)-1]
	if parent.kind == '{' {
		if !parent.expectObjectValue {
			return fmt.Errorf("snapshot JSON emitter expected an object key")
		}
		parent.expectObjectValue = false
		return nil
	}
	if !parent.first {
		if err := e.write([]byte{','}); err != nil {
			return err
		}
	}
	if err := e.write([]byte{'\n'}); err != nil {
		return err
	}
	if err := e.writeIndent(len(e.containers) * 2); err != nil {
		return err
	}
	parent.first = false
	e.arrayElements++
	if e.structuralErr == nil && e.arrayElements > e.limits.arrayElements {
		e.structuralErr = fmt.Errorf("actual-state snapshot exceeds the supported JSON array element limit of %s", snapshotJSONArrayElementLimitDisplay(e.limits.arrayElements))
	}
	return nil
}

func (e *snapshotJSONEmitter) beginObject() error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	if err := e.write([]byte{'{'}); err != nil {
		return err
	}
	e.token()
	e.containers = append(e.containers, snapshotJSONWriteContainer{kind: '{', first: true})
	return nil
}

func (e *snapshotJSONEmitter) endObject() error {
	if len(e.containers) == 0 || e.containers[len(e.containers)-1].kind != '{' {
		return fmt.Errorf("snapshot JSON emitter has an unmatched object")
	}
	container := e.containers[len(e.containers)-1]
	if container.expectObjectValue {
		return fmt.Errorf("snapshot JSON emitter has an object key without a value")
	}
	e.containers = e.containers[:len(e.containers)-1]
	if !container.first {
		if err := e.write([]byte{'\n'}); err != nil {
			return err
		}
		if err := e.writeIndent(len(e.containers) * 2); err != nil {
			return err
		}
	}
	if err := e.write([]byte{'}'}); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) beginArray() error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	if err := e.write([]byte{'['}); err != nil {
		return err
	}
	e.token()
	e.containers = append(e.containers, snapshotJSONWriteContainer{kind: '[', first: true})
	return nil
}

func (e *snapshotJSONEmitter) endArray() error {
	if len(e.containers) == 0 || e.containers[len(e.containers)-1].kind != '[' {
		return fmt.Errorf("snapshot JSON emitter has an unmatched array")
	}
	container := e.containers[len(e.containers)-1]
	e.containers = e.containers[:len(e.containers)-1]
	if !container.first {
		if err := e.write([]byte{'\n'}); err != nil {
			return err
		}
		if err := e.writeIndent(len(e.containers) * 2); err != nil {
			return err
		}
	}
	if err := e.write([]byte{']'}); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) key(key string) error {
	if len(e.containers) == 0 || e.containers[len(e.containers)-1].kind != '{' {
		return fmt.Errorf("snapshot JSON emitter expected an object")
	}
	object := &e.containers[len(e.containers)-1]
	if object.expectObjectValue {
		return fmt.Errorf("snapshot JSON emitter expected a value")
	}
	if !object.first {
		if err := e.write([]byte{','}); err != nil {
			return err
		}
	}
	if err := e.write([]byte{'\n'}); err != nil {
		return err
	}
	if err := e.writeIndent(len(e.containers) * 2); err != nil {
		return err
	}
	if err := e.quotedString(key); err != nil {
		return err
	}
	if err := e.write([]byte{':', ' '}); err != nil {
		return err
	}
	object.first = false
	object.expectObjectValue = true
	object.entries++
	e.token()
	if e.structuralErr == nil && object.entries > e.limits.objectEntriesPerObject {
		e.structuralErr = fmt.Errorf("actual-state snapshot exceeds the supported JSON object entry limit of %s", snapshotJSONObjectEntryLimitDisplay(e.limits.objectEntriesPerObject))
	}
	return nil
}

func (e *snapshotJSONEmitter) stringValue(value string) error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	if err := e.quotedString(value); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) quotedString(value string) error {
	if err := e.write([]byte{'"'}); err != nil {
		return err
	}
	scratch := snapshotJSONStringScratch{emitter: e}
	for index := 0; index < len(value); {
		b := value[index]
		if b < utf8.RuneSelf {
			if err := appendSnapshotJSONStringASCII(&scratch, b); err != nil {
				return err
			}
			index++
			continue
		}
		r, size := utf8.DecodeRuneInString(value[index:])
		if err := appendSnapshotJSONStringRune(&scratch, value[index:index+size], r, size); err != nil {
			return err
		}
		index += size
	}
	if err := scratch.flush(); err != nil {
		return err
	}
	return e.write([]byte{'"'})
}

type snapshotJSONStringScratch struct {
	emitter *snapshotJSONEmitter
	used    int
}

func (s *snapshotJSONStringScratch) append(data []byte) error {
	for len(data) > 0 {
		if s.used == len(s.emitter.stringScratch) {
			if err := s.flush(); err != nil {
				return err
			}
		}
		n := copy(s.emitter.stringScratch[s.used:], data)
		s.used += n
		data = data[n:]
	}
	return nil
}

func (s *snapshotJSONStringScratch) appendByte(value byte) error {
	if s.used == len(s.emitter.stringScratch) {
		if err := s.flush(); err != nil {
			return err
		}
	}
	s.emitter.stringScratch[s.used] = value
	s.used++
	return nil
}

func (s *snapshotJSONStringScratch) flush() error {
	if s.used == 0 {
		return nil
	}
	used := s.used
	s.used = 0
	return s.emitter.write(s.emitter.stringScratch[:used])
}

func appendSnapshotJSONStringASCII(scratch *snapshotJSONStringScratch, value byte) error {
	switch value {
	case '"':
		return scratch.append([]byte(`\"`))
	case '\\':
		return scratch.append([]byte(`\\`))
	case '\b':
		return scratch.append([]byte(`\b`))
	case '\f':
		return scratch.append([]byte(`\f`))
	case '\n':
		return scratch.append([]byte(`\n`))
	case '\r':
		return scratch.append([]byte(`\r`))
	case '\t':
		return scratch.append([]byte(`\t`))
	case '<':
		return scratch.append([]byte(`\u003c`))
	case '>':
		return scratch.append([]byte(`\u003e`))
	case '&':
		return scratch.append([]byte(`\u0026`))
	default:
		if value < 0x20 {
			const hex = "0123456789abcdef"
			return scratch.append([]byte{'\\', 'u', '0', '0', hex[value>>4], hex[value&0x0f]})
		}
		return scratch.appendByte(value)
	}
}

func appendSnapshotJSONStringRune(scratch *snapshotJSONStringScratch, encoded string, value rune, size int) error {
	if size == 1 && value == utf8.RuneError {
		return scratch.append([]byte(`\ufffd`))
	}
	switch value {
	case '\u2028':
		return scratch.append([]byte(`\u2028`))
	case '\u2029':
		return scratch.append([]byte(`\u2029`))
	default:
		return scratch.append([]byte(encoded))
	}
}

func (e *snapshotJSONEmitter) writeIndent(spaces int) error {
	var indent [64]byte
	if spaces > len(indent) {
		return fmt.Errorf("snapshot JSON nesting is too deep")
	}
	for i := range spaces {
		indent[i] = ' '
	}
	return e.write(indent[:spaces])
}

func (e *snapshotJSONEmitter) rawValue(value []byte) error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	if err := e.write(value); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) int64Value(value int64) error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	var buffer [32]byte
	encoded := strconv.AppendInt(buffer[:0], value, 10)
	if err := e.write(encoded); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) boolValue(value bool) error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	if value {
		if err := e.write([]byte("true")); err != nil {
			return err
		}
	} else if err := e.write([]byte("false")); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) nullValue() error {
	if err := e.beforeValue(); err != nil {
		return err
	}
	if err := e.write([]byte("null")); err != nil {
		return err
	}
	e.token()
	return nil
}

func (e *snapshotJSONEmitter) stringField(key, value string) error {
	if err := e.key(key); err != nil {
		return err
	}
	return e.stringValue(value)
}

func (e *snapshotJSONEmitter) int64Field(key string, value int64) error {
	if err := e.key(key); err != nil {
		return err
	}
	return e.int64Value(value)
}

func (e *snapshotJSONEmitter) boolField(key string, value bool) error {
	if err := e.key(key); err != nil {
		return err
	}
	return e.boolValue(value)
}

func (e *snapshotJSONEmitter) stringArray(values []string) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.stringValue(value); err != nil {
			return err
		}
	}
	return e.endArray()
}

func (e *snapshotJSONEmitter) int64Map(values map[string]int64) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginObject(); err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := e.key(key); err != nil {
			return err
		}
		if err := e.int64Value(values[key]); err != nil {
			return err
		}
	}
	return e.endObject()
}

func (e *snapshotJSONEmitter) memberArray(values []state.OrganizationMember) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.beginObject(); err != nil {
			return err
		}
		if err := e.int64Field("id", value.ID); err != nil {
			return err
		}
		if err := e.stringField("username", value.Username); err != nil {
			return err
		}
		if err := e.stringField("role", value.Role); err != nil {
			return err
		}
		if err := e.stringField("name", value.Name); err != nil {
			return err
		}
		if err := e.stringField("email", value.Email); err != nil {
			return err
		}
		if err := e.endObject(); err != nil {
			return err
		}
	}
	return e.endArray()
}

func (e *snapshotJSONEmitter) invitationArray(values []state.PendingInvitation) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.beginObject(); err != nil {
			return err
		}
		if err := e.int64Field("id", value.ID); err != nil {
			return err
		}
		if err := e.stringField("username", value.Username); err != nil {
			return err
		}
		if err := e.stringField("email", value.Email); err != nil {
			return err
		}
		if err := e.stringField("role", value.Role); err != nil {
			return err
		}
		if err := e.key("team_slugs"); err != nil {
			return err
		}
		if err := e.stringArray(value.TeamSlugs); err != nil {
			return err
		}
		if err := e.endObject(); err != nil {
			return err
		}
	}
	return e.endArray()
}

func (e *snapshotJSONEmitter) repositoryArray(values []state.Repository) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.beginObject(); err != nil {
			return err
		}
		if err := e.stringField("owner", value.Owner); err != nil {
			return err
		}
		if err := e.stringField("name", value.Name); err != nil {
			return err
		}
		if err := e.stringField("visibility", value.Visibility); err != nil {
			return err
		}
		if err := e.stringField("description", value.Description); err != nil {
			return err
		}
		if err := e.stringField("homepage", value.Homepage); err != nil {
			return err
		}
		if err := e.key("topics"); err != nil {
			return err
		}
		if err := e.stringArray(value.Topics); err != nil {
			return err
		}
		if err := e.boolField("allow_forking", value.AllowForking); err != nil {
			return err
		}
		if err := e.boolField("archived", value.Archived); err != nil {
			return err
		}
		if err := e.boolField("is_template", value.IsTemplate); err != nil {
			return err
		}
		if err := e.endObject(); err != nil {
			return err
		}
	}
	return e.endArray()
}

func (e *snapshotJSONEmitter) teamArray(values []state.Team) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.beginObject(); err != nil {
			return err
		}
		if err := e.int64Field("id", value.ID); err != nil {
			return err
		}
		if err := e.stringField("slug", value.Slug); err != nil {
			return err
		}
		if err := e.stringField("name", value.Name); err != nil {
			return err
		}
		if err := e.stringField("description", value.Description); err != nil {
			return err
		}
		if err := e.stringField("privacy", value.Privacy); err != nil {
			return err
		}
		if err := e.stringField("parent_slug", value.ParentSlug); err != nil {
			return err
		}
		if err := e.endObject(); err != nil {
			return err
		}
	}
	return e.endArray()
}

func (e *snapshotJSONEmitter) teamMemberArray(values []state.TeamMember) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.beginObject(); err != nil {
			return err
		}
		if err := e.stringField("team_slug", value.TeamSlug); err != nil {
			return err
		}
		if err := e.stringField("username", value.Username); err != nil {
			return err
		}
		if err := e.stringField("role", value.Role); err != nil {
			return err
		}
		if err := e.endObject(); err != nil {
			return err
		}
	}
	return e.endArray()
}

func (e *snapshotJSONEmitter) teamRepositoryPermissionArray(values []state.TeamRepositoryPermission) error {
	if values == nil {
		return e.nullValue()
	}
	if err := e.beginArray(); err != nil {
		return err
	}
	for _, value := range values {
		if err := e.beginObject(); err != nil {
			return err
		}
		if err := e.stringField("team_slug", value.TeamSlug); err != nil {
			return err
		}
		if err := e.stringField("owner", value.Owner); err != nil {
			return err
		}
		if err := e.stringField("name", value.Name); err != nil {
			return err
		}
		if err := e.stringField("permission", value.Permission); err != nil {
			return err
		}
		if err := e.endObject(); err != nil {
			return err
		}
	}
	return e.endArray()
}
