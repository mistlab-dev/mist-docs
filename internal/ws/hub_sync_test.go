package ws

import "testing"

func TestShouldPersistSync(t *testing.T) {
	step1 := []byte{MsgSync, SyncStep1, 1, 2}
	update := []byte{MsgSync, SyncUpdate, 9}
	awareness := []byte(`{"type":"awareness"}`)

	if !shouldPersistSync(false, step1) {
		t.Error("viewers must still catch up via step1")
	}
	if shouldPersistSync(false, update) {
		t.Error("viewers must not persist edits")
	}
	if !shouldPersistSync(true, update) {
		t.Error("editors persist updates")
	}
	if !shouldPersistSync(false, awareness) {
		t.Error("awareness is relayed for viewers")
	}
}
