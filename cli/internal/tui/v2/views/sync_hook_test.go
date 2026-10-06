package views

import (
	"context"
	"testing"
	"time"
)

func TestAfterTeamSyncRunsHook(t *testing.T) {
	got := make(chan string, 1)
	SetTeamSyncHook(func(_ context.Context, path string) { got <- path })
	t.Cleanup(func() { SetTeamSyncHook(nil) })

	AfterTeamSync(context.Background(), "")
	AfterTeamSync(context.Background(), "/ts")
	select {
	case p := <-got:
		if p != "/ts" {
			t.Fatalf("hook path = %q", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hook not called")
	}
}
