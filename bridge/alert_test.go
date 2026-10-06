package bridge

import "testing"

func TestAlertsFoldRepeatsAndKeepNewestFirst(t *testing.T) {
	alerts.list, alerts.open = nil, nil
	for i := 0; i < 5; i++ {
		raise("unknown-key", "203.0.113.9", 0, "a wrong key")
	}
	raise("duplicate", "198.51.100.4", 7, "client 7 from two places")
	raise("unknown-key", "192.0.2.1", 0, "another source")

	got := Alerts()
	if len(got) != 3 {
		t.Fatalf("want 3 alerts (five alike folded into one), got %d: %+v", len(got), got)
	}
	if got[0].IP != "192.0.2.1" || got[1].Kind != "duplicate" || got[1].Client != 7 {
		t.Fatalf("not newest first: %+v", got)
	}
	if got[2].IP != "203.0.113.9" || got[2].Count != 5 {
		t.Fatalf("the repeats were not counted into the first: %+v", got[2])
	}
}

func TestAlertsAreBounded(t *testing.T) {
	alerts.list, alerts.open = nil, nil
	for i := 0; i < alertKeep+50; i++ {
		raise("unknown-key", string(rune('a'+i%26))+string(rune('a'+i/26)), 0, "x")
	}
	if n := len(Alerts()); n != alertKeep {
		t.Fatalf("kept %d alerts, want %d", n, alertKeep)
	}
}
