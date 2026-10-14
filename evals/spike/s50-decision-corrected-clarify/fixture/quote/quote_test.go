package quote

import (
	"context"
	"testing"
	"time"
)

func TestWithDeadline(t *testing.T) {
	ctx, cancel := WithDeadline(context.Background())
	defer cancel()
	if d, ok := ctx.Deadline(); !ok || time.Until(d) > Deadline {
		t.Fatal("deadline")
	}
}
