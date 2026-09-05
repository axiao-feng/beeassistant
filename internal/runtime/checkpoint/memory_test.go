package checkpoint

import (
	"context"
	"sync"
	"testing"
)

func TestMemoryStoreCopiesValues(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	value := []byte("initial")
	if err := store.Set(ctx, "k", value); err != nil {
		t.Fatalf("set value: %v", err)
	}
	value[0] = 'x'

	got, ok, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get value: %v", err)
	}
	if !ok {
		t.Fatal("expected stored value")
	}
	if string(got) != "initial" {
		t.Fatalf("store should copy input value, got %q", got)
	}

	got[0] = 'y'
	gotAgain, ok, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get value again: %v", err)
	}
	if !ok {
		t.Fatal("expected stored value on second get")
	}
	if string(gotAgain) != "initial" {
		t.Fatalf("store should copy output value, got %q", gotAgain)
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if err := store.Set(ctx, "k", []byte("value")); err != nil {
					t.Errorf("set value: %v", err)
				}
				if _, _, err := store.Get(ctx, "k"); err != nil {
					t.Errorf("get value: %v", err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestMemoryStoreDeleteDropsState(t *testing.T) {
	store := NewMemoryStore().(*MemoryStore)
	ctx := context.Background()
	if err := store.Set(ctx, "run", []byte("state")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "run"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Get(ctx, "run"); err != nil || ok {
		t.Fatalf("deleted checkpoint = %v, %v", ok, err)
	}
}
