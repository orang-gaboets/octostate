package github

import (
	"time"

	gh "github.com/google/go-github/v55/github"
	"github.com/spf13/cobra"
)

// Ptr returns a pointer to the value of type T.
func Ptr[T any](v T) *T {
	return &v
}

// Unique returns a slice containing only the unique elements from the input slice.
func Unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{}, len(slice))
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// MergeUnique merges two slices and returns a new slice containing only unique elements.
func MergeUnique[T comparable](a, b []T) []T {
	seen := make(map[T]struct{}, len(a)+len(b))
	result := make([]T, 0, len(a)+len(b))
	for _, item := range a {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	for _, item := range b {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// ToSet converts a slice into a set represented as a map where keys are the unique elements
func ToSet[T comparable](xs []T) map[T]struct{} {
	m := make(map[T]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}

// EqualSets checks if two sets (represented as maps) are equal.
func EqualSets[T comparable](a, b map[T]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// MarkRequiredFlags marks the given flags as required for the provided command.
func MarkRequiredFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			cobra.CheckErr(err)
		}
	}
}

// TimestampToTime converts a GitHub Timestamp to a standard Go Time.
func TimestampToTime(ts *gh.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.Time
	return &t
}
