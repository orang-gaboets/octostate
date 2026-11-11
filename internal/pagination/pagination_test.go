package pagination

import (
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/pkg/github"
)

func TestNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		resp    *gh.Response
		current int
		want    int
		wantErr bool
	}{
		{name: "nil response ends", resp: nil, current: 0, want: 0},
		{name: "no next page ends", resp: &gh.Response{NextPage: 0}, current: 3, want: 0},
		{name: "first page advances", resp: &gh.Response{NextPage: 2}, current: 0, want: 2},
		{name: "later page advances", resp: &gh.Response{NextPage: 4}, current: 3, want: 4},
		{name: "first page repeated", resp: &gh.Response{NextPage: 1}, current: 0, wantErr: true},
		{name: "page repeated", resp: &gh.Response{NextPage: 3}, current: 3, wantErr: true},
		{name: "page regressed", resp: &gh.Response{NextPage: 2}, current: 3, wantErr: true},
		{name: "negative page", resp: &gh.Response{NextPage: -1}, current: 0, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Next(tt.resp, tt.current)
			if tt.wantErr {
				if !errors.Is(err, github.ErrValidationFailed) {
					t.Fatalf("Next() error = %v, want %v", err, github.ErrValidationFailed)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Next() = %d, %v; want %d, nil", got, err, tt.want)
			}
		})
	}
}

// drive runs the loop shape every caller uses over a scripted sequence of
// NextPage values, and reports how many pages were read.
func drive(nextPages []int) (int, error) {
	page, reads := 0, 0
	for {
		if reads == len(nextPages) {
			return reads, errors.New("script exhausted: loop did not stop")
		}
		resp := &gh.Response{NextPage: nextPages[reads]}
		reads++
		next, err := Next(resp, page)
		if err != nil {
			return reads, err
		}
		if next == 0 {
			return reads, nil
		}
		page = next
	}
}

func TestNextStopsLoops(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nextPages []int
		wantReads int
		wantErr   bool
	}{
		{name: "single page", nextPages: []int{0}, wantReads: 1},
		{name: "multiple pages", nextPages: []int{2, 3, 0}, wantReads: 3},
		{name: "repeated", nextPages: []int{2, 2, 2, 2}, wantReads: 2, wantErr: true},
		{name: "regressing", nextPages: []int{2, 3, 2, 3}, wantReads: 3, wantErr: true},
		{name: "cyclic back to first", nextPages: []int{2, 3, 1, 2}, wantReads: 3, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reads, err := drive(tt.nextPages)
			if reads != tt.wantReads {
				t.Fatalf("read %d pages, want %d", reads, tt.wantReads)
			}
			if tt.wantErr != errors.Is(err, github.ErrValidationFailed) || (!tt.wantErr && err != nil) {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
