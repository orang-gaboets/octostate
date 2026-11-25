package teamorigin

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/github"
)

func TestCanonicalizeMembershipsResolvesOriginPrecedenceIndependentOfInputOrder(t *testing.T) {
	rows := []Membership{
		{TeamSlug: "Platform", Username: "Alice", Role: "member", Origin: github.TeamMembershipOriginInherited},
		{TeamSlug: "platform", Username: "alice", Role: "member", Origin: github.TeamMembershipOriginUnknown},
		{TeamSlug: "PLATFORM", Username: "ALICE", Role: "member", Origin: github.TeamMembershipOriginDirect},
	}

	want := []Membership{{
		TeamSlug: "PLATFORM",
		Username: "ALICE",
		Role:     "member",
		Origin:   github.TeamMembershipOriginDirect,
	}}
	for _, permutation := range [][]Membership{
		rows,
		{rows[2], rows[0], rows[1]},
		{rows[1], rows[2], rows[0]},
	} {
		got, err := Canonicalize(permutation)
		if err != nil {
			t.Fatalf("Canonicalize returned error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Canonicalize = %#v, want %#v", got, want)
		}
	}
}

func TestCanonicalizeMembershipsRejectsConflictingRolesAtWinningOrigin(t *testing.T) {
	_, err := Canonicalize([]Membership{
		{TeamSlug: "platform", Username: "alice", Role: "member", Origin: github.TeamMembershipOriginDirect},
		{TeamSlug: "PLATFORM", Username: "ALICE", Role: "maintainer", Origin: github.TeamMembershipOriginDirect},
		{TeamSlug: "platform", Username: "alice", Role: "member", Origin: github.TeamMembershipOriginInherited},
	})
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
	}
}

func TestCanonicalizeMembershipsReportsConflictsDeterministically(t *testing.T) {
	rows := []Membership{
		{TeamSlug: "zeta", Username: "bob", Role: "member", Origin: github.TeamMembershipOriginDirect},
		{TeamSlug: "zeta", Username: "BOB", Role: "maintainer", Origin: github.TeamMembershipOriginDirect},
		{TeamSlug: "alpha", Username: "alice", Role: "member", Origin: github.TeamMembershipOriginDirect},
		{TeamSlug: "ALPHA", Username: "ALICE", Role: "maintainer", Origin: github.TeamMembershipOriginDirect},
	}
	_, firstErr := Canonicalize(rows)
	_, secondErr := Canonicalize([]Membership{rows[3], rows[1], rows[2], rows[0]})
	if firstErr == nil || secondErr == nil || firstErr.Error() != secondErr.Error() {
		t.Fatalf("conflict errors differ by input order: first=%v second=%v", firstErr, secondErr)
	}
	if !strings.Contains(firstErr.Error(), `"alpha"/"alice"`) {
		t.Fatalf("conflict error = %v, want deterministic first key alpha/alice", firstErr)
	}
}

func TestCanonicalizeMembershipsDefaultsEmptyOriginToUnknownAndRejectsInvalidOrigin(t *testing.T) {
	got, err := Canonicalize([]Membership{{TeamSlug: "platform", Username: "alice", Role: "member"}})
	if err != nil {
		t.Fatalf("Canonicalize legacy zero origin: %v", err)
	}
	want := []Membership{{TeamSlug: "platform", Username: "alice", Role: "member", Origin: github.TeamMembershipOriginUnknown}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Canonicalize legacy row = %#v, want %#v", got, want)
	}

	_, err = Canonicalize([]Membership{{TeamSlug: "platform", Username: "alice", Role: "member", Origin: github.TeamMembershipOrigin("maybe")}})
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("invalid origin error = %v, want %v", err, github.ErrValidationFailed)
	}
	_, err = Canonicalize([]Membership{{TeamSlug: "platform", Username: " ", Role: "member"}})
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("missing membership key error = %v, want %v", err, github.ErrValidationFailed)
	}
}
