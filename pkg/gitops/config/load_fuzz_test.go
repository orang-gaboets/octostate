package config

import (
	"bytes"
	"reflect"
	"testing"
)

// Run with:
// go test ./pkg/gitops/config -run=^$ -fuzz=FuzzDecodeYAML -fuzztime=30s
func FuzzDecodeYAML(f *testing.F) {
	f.Add([]byte("organization: orang-gaboets\n"))

	fullSeed, err := EncodeYAML(validOrganizationConfig())
	if err != nil {
		panic(err)
	}
	f.Add(fullSeed)

	f.Add([]byte(`
organization: orang-gaboets
unsupported: true
invites: []
repositories: []
teams: []
`))
	f.Add([]byte(`
organization: orang-gaboets
invites:
  - username: octocat
    username: duplicate
repositories: []
teams: []
`))
	f.Add([]byte(`
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    template:
      owner: orang-gaboets
      owner: other-org
      name: repo-template
teams: []
invites: []
`))
	f.Add([]byte(`
organization: orang-gaboets
invites:
  - role: direct_member
repositories:
  - name: octostate
    visibility: private
teams: []
`))
	f.Add([]byte(`
organization: orang-gaboets
invites:
  - username: null
    email: null
    user_id: null
    role: direct_member
repositories:
  - name: octostate
    visibility: private
    description: null
    homepage: null
    allow_forking: null
    archived: null
    is_template: null
teams: []
`))
	f.Add([]byte(`
organization: orang-gaboets
invites:
  - username: [octocat]
    role: direct_member
repositories: []
teams: []
`))
	f.Add([]byte(`
organization: orang-gaboets
---
organization: other-org
`))
	f.Add([]byte(`
organization: !!str orang-gaboets
repositories:
  - name: octostate
    owner: &org orang-gaboets
    visibility: private
    template:
      owner: *org
      name: repo-template
teams: []
invites: []
`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var first OrganizationConfig
		if err := decodeYAML(bytes.NewReader(data), &first); err != nil {
			return
		}
		normalize(&first)

		var second OrganizationConfig
		if err := decodeYAML(bytes.NewReader(data), &second); err != nil {
			t.Fatalf("second decode failed: %v", err)
		}
		normalize(&second)

		if !reflect.DeepEqual(first, second) {
			t.Fatalf("non-deterministic decode: first=%#v second=%#v", first, second)
		}
	})
}
