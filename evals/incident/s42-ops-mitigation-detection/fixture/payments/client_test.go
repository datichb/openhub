package payments

import (
	"errors"
	"testing"
)

type flaky struct{ fails, calls int }

func (f *flaky) Charge(string, int64) error {
	f.calls++
	if f.calls <= f.fails {
		return errors.New("refused")
	}
	return nil
}

func TestCharge(t *testing.T) {
	p := &flaky{}
	if err := Charge(p, "ord-1", 1000); err != nil || p.calls != 1 {
		t.Fatalf("err = %v, calls = %d", err, p.calls)
	}
}
