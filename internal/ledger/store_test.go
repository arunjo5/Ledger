package ledger

import (
	"context"
	"errors"
	"testing"
)

func ptr(s string) *string { return &s }

func TestCreateAccountWithLabel(t *testing.T) {
	store := newTestStore(t)
	acc, err := store.CreateAccount(context.Background(), ptr("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if acc.ID == "" {
		t.Fatal("expected a generated id")
	}
	if acc.Label == nil || *acc.Label != "alice" {
		t.Fatalf("label = %v, want alice", acc.Label)
	}
}

func TestCreateAccountWithoutLabel(t *testing.T) {
	store := newTestStore(t)
	acc, err := store.CreateAccount(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Label != nil {
		t.Fatalf("label = %v, want nil", *acc.Label)
	}
}

func TestDuplicateLabelRejected(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if _, err := store.CreateAccount(ctx, ptr("cash")); err != nil {
		t.Fatal(err)
	}
	_, err := store.CreateAccount(ctx, ptr("cash"))
	if !errors.Is(err, ErrDuplicateLabel) {
		t.Fatalf("err = %v, want ErrDuplicateLabel", err)
	}
}

func TestGetAccount(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	created, err := store.CreateAccount(ctx, ptr("bob"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetAccount(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("id = %s, want %s", got.ID, created.ID)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetAccount(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListAccounts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	for _, label := range []string{"a", "b", "c"} {
		if _, err := store.CreateAccount(ctx, ptr(label)); err != nil {
			t.Fatal(err)
		}
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 3 {
		t.Fatalf("len = %d, want 3", len(accounts))
	}
}
