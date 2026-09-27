package tests

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/middleware"
	approuter "github.com/c-wind/mist-docs/internal/router"
	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsHub *ws.Hub

// wsServer serves the collaboration endpoint exactly as main.go mounts it.
func wsServer(t *testing.T) *httptest.Server {
	t.Helper()
	if wsHub == nil {
		wsHub = ws.NewHub()
		go wsHub.Run()
	}
	r := gin.New()
	approuter.RegisterWS(r, wsHub)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func wsDial(t *testing.T, srv *httptest.Server, docID, token string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/teams/" + teamID + "/docs/" + docID + "?token=" + token
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		t.Fatalf("dial: %v (http %d)", err, code)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// wsTempMember adds a throwaway editor so removing them does not disturb
// the shared fixtures.
func wsTempMember(t *testing.T, id string) string {
	t.Helper()
	mustExec(t, `INSERT INTO users (id, email, username, display_name, password_hash, is_admin, email_verified)
		VALUES (?, ?, ?, ?, 'x', 0, 1)`, id, id+"@mistdocs.invalid", id, id)
	mustExec(t, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'editor')`, teamID, id)
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM team_members WHERE user_id=?`, id)
		database.DB.Exec(`DELETE FROM users WHERE id=?`, id)
	})
	token, _ := middleware.GenerateToken(id, id, "member", "")
	return token
}

// waitClosed reads until the server closes the connection and returns the
// close code, or fails after timeout.
func waitClosed(t *testing.T, conn *websocket.Conn, timeout time.Duration) int {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			return ce.Code
		}
		t.Fatalf("connection still open or failed without close frame: %v", err)
	}
}

// A member removed from the team in the Portal kept an open editing session
// forever: the periodic check reused the role cached at connect time.
func TestWSRemovedMemberIsDisconnected(t *testing.T) {
	restore := ws.SetAccessCheckIntervals(200*time.Millisecond, time.Hour)
	defer restore()

	docID := createTestDoc(t, adminToken, "ws-removed-member")
	token := wsTempMember(t, "test-user-ws-kick")
	srv := wsServer(t)
	conn := wsDial(t, srv, docID, token)

	mustExec(t, `DELETE FROM team_members WHERE team_id=? AND user_id=?`, teamID, "test-user-ws-kick")

	if code := waitClosed(t, conn, 3*time.Second); code != ws.CloseAccessRevoked {
		t.Fatalf("close code = %d, want %d", code, ws.CloseAccessRevoked)
	}
}

// Between two timer ticks an edit must still be checked before it is
// accepted and relayed to the other people in the document.
func TestWSEditFromRemovedMemberIsRejected(t *testing.T) {
	restore := ws.SetAccessCheckIntervals(time.Hour, 0)
	defer restore()

	docID := createTestDoc(t, adminToken, "ws-removed-edit")
	token := wsTempMember(t, "test-user-ws-edit")
	srv := wsServer(t)
	watcher := wsDial(t, srv, docID, adminToken)
	editor := wsDial(t, srv, docID, token)
	time.Sleep(100 * time.Millisecond) // let both joins settle

	mustExec(t, `DELETE FROM team_members WHERE team_id=? AND user_id=?`, teamID, "test-user-ws-edit")
	update := []byte{ws.MsgSync, ws.SyncUpdate, 1, 2, 3}
	if err := editor.WriteMessage(websocket.BinaryMessage, update); err != nil {
		t.Fatalf("write: %v", err)
	}
	if code := waitClosed(t, editor, 3*time.Second); code != ws.CloseAccessRevoked {
		t.Fatalf("close code = %d, want %d", code, ws.CloseAccessRevoked)
	}

	// The watcher may see join/leave/clients JSON, but never the update.
	watcher.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		_, data, err := watcher.ReadMessage()
		if err != nil {
			break
		}
		if len(data) >= 2 && data[0] == ws.MsgSync && data[1] == ws.SyncUpdate {
			t.Fatalf("update from a removed member was relayed: %v", data)
		}
	}
}

// Members who still belong to the team keep their session across checks.
func TestWSMemberStaysConnected(t *testing.T) {
	restore := ws.SetAccessCheckIntervals(100*time.Millisecond, 0)
	defer restore()

	docID := createTestDoc(t, adminToken, "ws-member-stays")
	srv := wsServer(t)
	conn := wsDial(t, srv, docID, editorToken)

	conn.SetReadDeadline(time.Now().Add(600 * time.Millisecond))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			t.Fatalf("member was disconnected: %v", err)
		}
		break // read deadline: still connected
	}
}
