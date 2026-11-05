package gns3

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestListSnapshots(t *testing.T) {
	got, err := replay(t, "snapshots/list").ListSnapshots(context.Background(), "1f3db850-1612-46a1-8fce-127b55f7e606")
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{
		SnapshotID: "d0b7e0d7-406e-4a57-90e3-2166ccf32e2f", ProjectID: "1f3db850-1612-46a1-8fce-127b55f7e606",
		Name: "snap1", CreatedAt: 1793847731,
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want [%+v]", got, want)
	}
}

func TestCreateSnapshot(t *testing.T) {
	s, err := replay(t, "snapshots/create").CreateSnapshot(context.Background(), "1f3db850-1612-46a1-8fce-127b55f7e606", "snap1")
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{
		SnapshotID: "d0b7e0d7-406e-4a57-90e3-2166ccf32e2f", ProjectID: "1f3db850-1612-46a1-8fce-127b55f7e606",
		Name: "snap1", CreatedAt: 1793847731,
	}
	if *s != want {
		t.Fatalf("got %+v, want %+v", *s, want)
	}
}

func TestRestoreSnapshot(t *testing.T) {
	err := replay(t, "snapshots/restore").RestoreSnapshot(context.Background(),
		"1f3db850-1612-46a1-8fce-127b55f7e606", "d0b7e0d7-406e-4a57-90e3-2166ccf32e2f")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRestoreSnapshotNotFound(t *testing.T) {
	err := replay(t, "snapshots/restore_404").RestoreSnapshot(context.Background(),
		"1f3db850-1612-46a1-8fce-127b55f7e606", "00000000-0000-0000-0000-000000000000")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want 404", err)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	err := replay(t, "snapshots/delete").DeleteSnapshot(context.Background(),
		"1f3db850-1612-46a1-8fce-127b55f7e606", "d0b7e0d7-406e-4a57-90e3-2166ccf32e2f")
	if err != nil {
		t.Fatal(err)
	}
}

// TestCreateSnapshotDuplicateName: 2.2.61 refuses a second snapshot with the
// same name (not in the docs, docs/api-drift.md).
func TestCreateSnapshotDuplicateName(t *testing.T) {
	_, err := replay(t, "snapshots/create_409").CreateSnapshot(context.Background(), "1f3db850-1612-46a1-8fce-127b55f7e606", "snap1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 || !strings.Contains(err.Error(), "The snapshot name snap1 already exists") {
		t.Fatalf("err = %v, want 409 naming the duplicate", err)
	}
}
