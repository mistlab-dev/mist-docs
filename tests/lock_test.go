package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gorilla/websocket"
)

// Bug 3: locks were invisible to other users (no locked_by in responses) and
// neither the API nor live editing honoured them.
func TestLockIsVisibleAndEnforced(t *testing.T) {
	_, editor2Tok := roleFixtures(t)
	doc := createTestDoc(t, adminToken, "锁定测试")
	// a second version so restore has something to go back to
	request("PUT", teamPath("/documents/"+doc+"/content"), map[string]string{"content": "<p>v2</p>"}, adminToken)

	w := request("POST", teamPath("/documents/"+doc+"/lock"), nil, editorToken)
	if w.Code != 200 {
		t.Fatalf("lock: %d %s", w.Code, w.Body.String())
	}
	if got := getString(parseJSON(t, w)["locked_by"]); got != editorID {
		t.Fatalf("lock response locked_by=%q", got)
	}

	// Everyone reading the document sees who holds the lock.
	for _, path := range []string{"/documents/" + doc, "/documents/" + doc + "/content"} {
		w = request("GET", teamPath(path), nil, editor2Tok)
		body := parseJSON(t, w)
		data, _ := body["data"].(map[string]interface{})
		if data == nil {
			data = body
		}
		if getString(data["locked_by"]) != editorID || getString(data["locked_by_name"]) == "" {
			t.Fatalf("GET %s: locked_by=%v name=%v", path, data["locked_by"], data["locked_by_name"])
		}
	}

	// Another editor cannot take, bypass or release the lock.
	w = request("POST", teamPath("/documents/"+doc+"/lock"), nil, editor2Tok)
	if w.Code != 409 || getString(parseJSON(t, w)["locked_by"]) != editorID {
		t.Fatalf("second lock = %d %s, want 409 with locked_by", w.Code, w.Body.String())
	}
	if w = request("PUT", teamPath("/documents/"+doc+"/content"), map[string]string{"content": "<p>bypass</p>"}, editor2Tok); w.Code != 409 {
		t.Fatalf("save while locked by someone else = %d, want 409", w.Code)
	}
	if w = request("POST", teamPath("/documents/"+doc+"/restore"), map[string]int{"version": 1}, editor2Tok); w.Code != 409 {
		t.Fatalf("restore while locked = %d, want 409", w.Code)
	}
	if w = request("POST", teamPath("/documents/"+doc+"/unlock"), nil, editor2Tok); w.Code != 403 {
		t.Fatalf("unlock by non-holder = %d, want 403", w.Code)
	}

	// The holder can still save; an admin can override and release.
	if w = request("PUT", teamPath("/documents/"+doc+"/content"), map[string]string{"content": "<p>holder</p>"}, editorToken); w.Code != 200 {
		t.Fatalf("holder save = %d", w.Code)
	}
	if w = request("PUT", teamPath("/documents/"+doc+"/content"), map[string]string{"content": "<p>admin</p>"}, adminToken); w.Code != 200 {
		t.Fatalf("admin override save = %d", w.Code)
	}
	if w = request("POST", teamPath("/documents/"+doc+"/unlock"), nil, adminToken); w.Code != 200 {
		t.Fatalf("admin unlock = %d", w.Code)
	}
	if w = request("PUT", teamPath("/documents/"+doc+"/content"), map[string]string{"content": "<p>free</p>"}, editor2Tok); w.Code != 200 {
		t.Fatalf("save after unlock = %d", w.Code)
	}
}

// Live edits used to bypass the lock completely.
func TestWSLockBlocksLiveEdits(t *testing.T) {
	restore := ws.SetAccessCheckIntervals(time.Hour, 0)
	defer restore()
	_, editor2Tok := roleFixtures(t)

	doc := createTestDoc(t, adminToken, "ws-lock")
	srv := wsServer(t)
	watcher := wsDial(t, srv, doc, editorToken)
	other := wsDial(t, srv, doc, editor2Tok)
	time.Sleep(100 * time.Millisecond)

	if w := request("POST", teamPath("/documents/"+doc+"/lock"), nil, editorToken); w.Code != 200 {
		t.Fatalf("lock: %d", w.Code)
	}

	// The other editor is told right away that it lost write access.
	other.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err := other.ReadMessage()
		if err != nil {
			t.Fatalf("no permission message after lock: %v", err)
		}
		// control messages are JSON in binary frames, like every other frame
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil && m["type"] == "permission" {
			if m["can_write"] != false || m["locked_by"] != editorID {
				t.Fatalf("permission message = %v", m)
			}
			break
		}
	}

	// Its updates are no longer relayed to the lock holder.
	other.WriteMessage(websocket.BinaryMessage, []byte{ws.MsgSync, ws.SyncUpdate, 9, 9, 9})
	watcher.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		_, data, err := watcher.ReadMessage()
		if err != nil {
			break
		}
		if len(data) >= 2 && data[0] == ws.MsgSync && data[1] == ws.SyncUpdate {
			t.Fatalf("update from a locked-out editor was relayed: %v", data)
		}
	}

	// Unlocking gives write access back.
	request("POST", teamPath("/documents/"+doc+"/unlock"), nil, editorToken)
	other.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err := other.ReadMessage()
		if err != nil {
			t.Fatalf("no permission message after unlock: %v", err)
		}
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil && m["type"] == "permission" {
			if m["can_write"] != true {
				t.Fatalf("after unlock: %v", m)
			}
			return
		}
	}
}
