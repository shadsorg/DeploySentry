package models

import "testing"

func TestCoordinationPolicyStrings(t *testing.T) {
	for p, want := range map[CoordinationPolicy]string{
		CoordinationIndependent:         "independent",
		CoordinationPauseOnSiblingAbort: "pause_on_sibling_abort",
		CoordinationCascadeAbort:        "cascade_abort",
	} {
		if string(p) != want {
			t.Errorf("%v: got %q want %q", p, string(p), want)
		}
	}
}
