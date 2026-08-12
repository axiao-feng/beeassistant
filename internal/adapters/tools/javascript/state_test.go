package javascript

import (
	"strings"
	"testing"
)

func TestFileStateStorePersistsNamespaces(t *testing.T) {
	store, err := NewFileStateStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStateStore() error = %v", err)
	}
	if err := store.Set("first", "count", map[string]any{"value": 2}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, exists, err := store.Get("first", "count")
	if err != nil || !exists {
		t.Fatalf("Get() = %#v/%v/%v", value, exists, err)
	}
	object, ok := value.(map[string]any)
	if !ok || object["value"] != float64(2) {
		t.Fatalf("Get() value = %#v", value)
	}
	if _, exists, err := store.Get("second", "count"); err != nil || exists {
		t.Fatalf("Get(second) exists/error = %v/%v", exists, err)
	}
	keys, err := store.Keys("first")
	if err != nil || len(keys) != 1 || keys[0] != "count" {
		t.Fatalf("Keys() = %#v/%v", keys, err)
	}
	deleted, err := store.Delete("first", "count")
	if err != nil || !deleted {
		t.Fatalf("Delete() = %v/%v", deleted, err)
	}
}

func TestFileStateStoreEnforcesSizeLimit(t *testing.T) {
	store, err := NewFileStateStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStateStore() error = %v", err)
	}
	err = store.Set("large", "value", strings.Repeat("x", maxStateFileBytes))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Set() error = %v, want size limit", err)
	}
}
