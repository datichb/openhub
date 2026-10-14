package counter

import (
	"reflect"
	"testing"
)

func TestIncGet(t *testing.T) {
	c := New()
	c.Inc("a")
	c.Inc("a")
	c.Inc("b")
	if got := c.Get("a"); got != 2 {
		t.Fatalf("Get(a) = %d, want 2", got)
	}
	if got := c.Get("z"); got != 0 {
		t.Fatalf("Get(z) = %d, want 0", got)
	}
}

func TestKeys(t *testing.T) {
	c := New()
	c.Inc("b")
	c.Inc("a")
	if got := c.Keys(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("Keys() = %v", got)
	}
}
