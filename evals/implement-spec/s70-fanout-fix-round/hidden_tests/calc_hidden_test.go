package calc

import (
	"os"
	"strings"
	"testing"
)

func TestHiddenMulAndFix(t *testing.T) {
	for _, c := range []struct{ a, b, want int }{{4, 3, 12}, {-2, 3, -6}, {0, 7, 0}} {
		if got := Mul(c.a, c.b); got != c.want {
			t.Errorf("Mul(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	data, err := os.ReadFile("mul_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "func TestMulZero") {
		t.Error("the fix round asked for TestMulZero in mul_test.go")
	}
}
