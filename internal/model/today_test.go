package model

import (
	"testing"
	"time"
)

func TestDayOfUsesBusinessTimezone(t *testing.T) {
	defer SetTimezone("")
	// 2026-09-27 22:30 in New York is already 2026-09-28 in Shanghai.
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata missing")
	}
	at := time.Date(2026, 9, 27, 22, 30, 0, 0, ny)

	if err := SetTimezone("Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	got := DayOf(at)
	if got.Format("2006-01-02") != "2026-09-28" {
		t.Fatalf("Shanghai day = %s, want 2026-09-28", got.Format("2006-01-02"))
	}
	if got.Location() != time.Local || got.Hour() != 0 {
		t.Fatalf("DayOf must be local midnight (DATE columns parse as local), got %v", got)
	}

	if err := SetTimezone("America/New_York"); err != nil {
		t.Fatal(err)
	}
	if d := DayOf(at).Format("2006-01-02"); d != "2026-09-27" {
		t.Fatalf("New York day = %s, want 2026-09-27", d)
	}
	if err := SetTimezone("Not/AZone"); err == nil {
		t.Fatal("invalid zone accepted")
	}
}
