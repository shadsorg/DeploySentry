package models

import "testing"

func TestRolloutPolicyKind(t *testing.T) {
	cases := []struct {
		p    PolicyKind
		want string
	}{{PolicyOff, "off"}, {PolicyPrompt, "prompt"}, {PolicyMandate, "mandate"}}
	for _, c := range cases {
		if string(c.p) != c.want {
			t.Errorf("PolicyKind %v: got %q want %q", c.p, string(c.p), c.want)
		}
	}
}
