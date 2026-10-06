package workflow

import "testing"

func TestPickRuntime(t *testing.T) {
	sp := &Spec{Runtime: &RuntimeSpec{Default: RuntimeLocal, Allowed: []Runtime{RuntimeLocal, RuntimeContainer}}}
	cases := []struct {
		prefs []string
		want  Runtime
	}{
		{nil, RuntimeLocal},
		{[]string{"container"}, RuntimeContainer},
		{[]string{"", "container"}, RuntimeContainer},       // project unset: settings
		{[]string{"remote", "container"}, RuntimeContainer}, // not allowed: next
		{[]string{"local", "container"}, RuntimeLocal},      // most specific first
		{[]string{"remote", "bogus"}, RuntimeLocal},         // none allowed: workflow default
	}
	for _, c := range cases {
		if got := sp.PickRuntime(c.prefs...); got != c.want {
			t.Errorf("PickRuntime(%q) = %s, want %s", c.prefs, got, c.want)
		}
	}
	if got := (&Spec{}).PickRuntime("container"); got != RuntimeLocal {
		t.Errorf("no runtime block: %s, want local", got)
	}
}
