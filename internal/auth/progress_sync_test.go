package auth

import (
	"context"
	"testing"
)

func TestProgressDeletionSyncDoesNotBlockNewReading(t *testing.T) {
	ctx := context.Background()
	a, b := newService(t), newService(t)
	ua, err := a.CreateUser(ctx, "a", "password123", "", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	ub, err := b.CreateUser(ctx, "b", "password123", "", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveProgress(ctx, ua.ID, 7, Progress{Overall: 0.2}); err != nil {
		t.Fatal(err)
	}
	old, err := a.ExportState(ctx, ua.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.MergeState(ctx, ub.ID, old); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteProgress(ctx, ua.ID, 7); err != nil {
		t.Fatal(err)
	}
	removed, err := a.ExportState(ctx, ua.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed.Progress) != 1 || !removed.Progress[0].Deleted {
		t.Fatalf("deletion not exported: %+v", removed.Progress)
	}
	if err := b.MergeState(ctx, ub.ID, removed); err != nil {
		t.Fatal(err)
	}
	if err := b.MergeState(ctx, ub.ID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetProgress(ctx, ub.ID, 7); err != ErrNotFound {
		t.Fatalf("old snapshot resurrected progress: %v", err)
	}
	// A genuine new device push must clear the deletion and win on both sides.
	if err := b.SaveOverall(ctx, ub.ID, 7, 0.4); err != nil {
		t.Fatal(err)
	}
	resumed, err := b.ExportState(ctx, ub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.Progress) != 1 || resumed.Progress[0].Deleted {
		t.Fatalf("new reading blocked: %+v", resumed.Progress)
	}
	if err := a.MergeState(ctx, ua.ID, resumed); err != nil {
		t.Fatal(err)
	}
	if err := a.MergeState(ctx, ua.ID, removed); err != nil {
		t.Fatal(err)
	}
	if p, err := a.GetProgress(ctx, ua.ID, 7); err != nil || p.Overall != 0.4 {
		t.Fatalf("new progress lost: %+v %v", p, err)
	}
	// Deleting an unknown book should not accumulate meaningless deletion rows.
	if err := a.DeleteProgress(ctx, ua.ID, 999); err != nil {
		t.Fatal(err)
	}
	state, err := a.ExportState(ctx, ua.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Progress) != 1 {
		t.Fatalf("unknown deletion polluted state: %+v", state.Progress)
	}
}

func TestKOReaderProgressInvalidatesWebPosition(t *testing.T) {
	ctx := context.Background()
	service := newService(t)
	user, err := service.CreateUser(ctx, "reader", "password123", "", RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveProgress(ctx, user.ID, 7, Progress{Chapter: 10, Position: 0.5, Overall: 0.7}); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveOverall(ctx, user.ID, 7, 0.03); err != nil {
		t.Fatal(err)
	}
	progress, err := service.GetProgress(ctx, user.ID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Overall != 0.03 || progress.Locator != "kosync:overall" {
		t.Fatalf("device update: %+v", progress)
	}
	// Reading backwards is allowed; the fix must not become a maximum-percent rule.
	if err := service.SaveProgress(ctx, user.ID, 7, Progress{Chapter: 0, Position: 0.1, Overall: 0.01}); err != nil {
		t.Fatal(err)
	}
	progress, err = service.GetProgress(ctx, user.ID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Overall != 0.01 || progress.Locator != "" {
		t.Fatalf("new web reading: %+v", progress)
	}
}
