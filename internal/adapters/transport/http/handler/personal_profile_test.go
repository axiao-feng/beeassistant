package handler

import (
	"testing"
)

func TestPersonalProfileStoreRoundTrip(t *testing.T) {
	store := NewPersonalProfileStore(t.TempDir())
	want := PersonalProfile{
		Name:       "小明",
		Occupation: "独立开发者",
		Location:   "深圳",
		Goals:      "完成个人综合助手",
		Notes:      "中文回答，结论优先",
	}

	saved, err := store.Save(want)
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}
	if saved.UpdatedAt.IsZero() {
		t.Fatal("saved profile should have updated_at")
	}

	got, err := store.Get()
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	want.UpdatedAt = got.UpdatedAt
	if got != want {
		t.Fatalf("profile = %#v, want %#v", got, want)
	}
}
