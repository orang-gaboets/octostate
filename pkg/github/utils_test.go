package github

import (
	"reflect"
	"testing"
)

func TestUniqueStableOrder(t *testing.T) {
	input := []int{1, 2, 1, 3, 2}
	want := []int{1, 2, 3}
	got := Unique(input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestMergeUniqueStableOrder(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"c", "d", "b", "e"}
	want := []string{"a", "b", "c", "d", "e"}
	got := MergeUnique(a, b)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
