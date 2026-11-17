package main

import "testing"

func TestDiffSet(t *testing.T) {
	added, removed := diffSet([]string{"a", "b"}, []string{"b", "c"})
	if len(added) != 1 || added[0] != "c" || len(removed) != 1 || removed[0] != "a" {
		t.Fatalf("diffSet = %v / %v", added, removed)
	}
}
