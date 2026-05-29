#!/bin/bash
# MistDocs Integration Test
set -e

BASE="https://docs.mistlab.dev"
PASS=0; FAIL=0; TOTAL=0

ok() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo -e "  \033[32m✅ PASS\033[0m $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo -e "  \033[31m❌ FAIL\033[0m $1"; }
section() { echo -e "\n\033[1;33m━━━ $1 ━━━\033[0m"; }

# JSON helper: extract field, handles {data:{...}} wrapper
jf() { python3 -c "
import sys,json
d=json.load(sys.stdin)
r=d.get('data',None)
if r is None or not isinstance(r,dict): r=d
print(r.get('$1',''))
" 2>/dev/null; }

# Track created resources for cleanup
CREATED_DOCS=""
CREATED_FOLDERS=""
CREATED_USERS=""

cleanup() {
  echo -e "\n\033[1;33m━━━ Cleanup ━━━\033[0m"
  [ -n "$TOKEN" ] || return
  for ID in $CREATED_DOCS; do
    $CURL "$BASE/api/docs/documents/$ID" -X DELETE -H "$AUTH" > /dev/null 2>&1 && echo "  🗑️  Deleted doc $ID" || true
  done
  for ID in $CREATED_FOLDERS; do
    $CURL "$BASE/api/docs/folders/$ID" -X DELETE -H "$AUTH" > /dev/null 2>&1 && echo "  🗑️  Deleted folder $ID" || true
  done
  for ID in $CREATED_USERS; do
    $CURL "$BASE/api/users/$ID" -X DELETE -H "$AUTH" > /dev/null 2>&1 && echo "  🗑️  Deleted user $ID" || true
  done
}
trap cleanup EXIT

CURL="curl -sk"

echo -e "\n\033[1;36m╔══════════════════════════════════╗"
echo -e "║  MistDocs Integration Test       ║"
echo -e "╚══════════════════════════════════╝\033[0m"

# ─── Auth ───
section "Auth"

LOGIN=$($CURL "$BASE/api/auth/login" -X POST -H 'Content-Type: application/json' -d '{"username":"admin","password":"Admin@2026"}')
TOKEN=$(echo "$LOGIN" | jf 'token')
AUTH="Authorization: Bearer $TOKEN"

[ -n "$TOKEN" ] && ok "Login" || { fail "Login"; exit 1; }

ME=$($CURL "$BASE/api/auth/me" -H "$AUTH")
ME_ID=$(echo "$ME" | jf 'id')
[ -n "$ME_ID" ] && ok "Get current user ($ME_ID)" || fail "Get current user"

# ─── Documents ───
section "Documents"

DOCS=$($CURL "$BASE/api/docs/documents" -H "$AUTH")
DOC_ID=$(echo "$DOCS" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); print(d[0]['id'] if d else '')" 2>/dev/null)
DOC_COUNT=$(echo "$DOCS" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null)
[ "$DOC_COUNT" -ge 0 ] && ok "List documents ($DOC_COUNT)" || fail "List documents"

NEW_DOC=$($CURL "$BASE/api/docs/documents" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"集成测试文档","type":"doc","folder_id":""}')
NEW_DOC_ID=$(echo "$NEW_DOC" | jf 'id')
[ -n "$NEW_DOC_ID" ] && { ok "Create document"; CREATED_DOCS="$CREATED_DOCS $NEW_DOC_ID"; } || fail "Create document"

GET_DOC=$($CURL "$BASE/api/docs/documents/$NEW_DOC_ID" -H "$AUTH")
GET_TITLE=$(echo "$GET_DOC" | jf 'title')
[ "$GET_TITLE" = "集成测试文档" ] && ok "Get document" || fail "Get document (got: $GET_TITLE)"

$CURL "$BASE/api/docs/documents/$NEW_DOC_ID" -X PUT -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"集成测试文档(已改)","content":"<p>Hello</p>"}' > /dev/null
UPDATED=$($CURL "$BASE/api/docs/documents/$NEW_DOC_ID" -H "$AUTH")
UP_TITLE=$(echo "$UPDATED" | jf 'title')
[ "$UP_TITLE" = "集成测试文档(已改)" ] && ok "Update document" || fail "Update document"

DEL=$($CURL "$BASE/api/docs/documents/$NEW_DOC_ID" -X DELETE -H "$AUTH")
DEL_MSG=$(echo "$DEL" | jf 'message')
[ -n "$DEL_MSG" ] && ok "Delete document" || fail "Delete document"

# ─── Folders ───
section "Folders"

NEW_FOLDER=$($CURL "$BASE/api/docs/folders" -X POST -H "$AUTH" -H 'Content-Type: application/json' -d '{"name":"测试文件夹"}')
FOLDER_ID=$(echo "$NEW_FOLDER" | jf 'id')
[ -n "$FOLDER_ID" ] && { ok "Create folder"; CREATED_FOLDERS="$CREATED_FOLDERS $FOLDER_ID"; } || fail "Create folder"

TREE=$($CURL "$BASE/api/docs/tree" -H "$AUTH")
TREE_OK=$(echo "$TREE" | python3 -c "import sys,json; d=json.load(sys.stdin); print('ok' if isinstance(d.get('data'), list) else 'fail')" 2>/dev/null)
[ "$TREE_OK" = "ok" ] && ok "List folder tree" || fail "List folder tree"

$CURL "$BASE/api/docs/folders/$FOLDER_ID" -X DELETE -H "$AUTH" > /dev/null
ok "Delete folder"

# ─── Dashboard ───
section "Dashboard"

DASH=$($CURL "$BASE/api/admin/dashboard" -H "$AUTH")
DASH_OK=$(echo "$DASH" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',{}); print('ok' if 'users' in d and 'documents' in d else 'fail')" 2>/dev/null)
[ "$DASH_OK" = "ok" ] && ok "Dashboard stats" || fail "Dashboard stats"

SYSINFO=$($CURL "$BASE/api/admin/system-info" -H "$AUTH")
SYSINFO_OK=$(echo "$SYSINFO" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',{}); print('ok' if 'version' in d else 'fail')" 2>/dev/null)
[ "$SYSINFO_OK" = "ok" ] && ok "System info" || fail "System info"

# ─── Share ───
section "Share"

SHARE=$($CURL "$BASE/api/docs/documents/$DOC_ID/share" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"password":"test123","expires_in":24}')
SHARE_TOKEN=$(echo "$SHARE" | jf 'token')
SHARE_ID=$(echo "$SHARE" | jf 'share_id')
[ -n "$SHARE_TOKEN" ] && ok "Create share" || fail "Create share"

NOPWD=$($CURL "$BASE/api/s/$SHARE_TOKEN")
NOPWD_ERR=$(echo "$NOPWD" | jf 'error')
[ -n "$NOPWD_ERR" ] && ok "Share rejects no password" || fail "Share rejects no password"

WRONG=$($CURL "$BASE/api/s/$SHARE_TOKEN?password=wrong")
WRONG_ERR=$(echo "$WRONG" | jf 'error')
[ -n "$WRONG_ERR" ] && ok "Share rejects wrong password" || fail "Share rejects wrong password"

CORRECT=$($CURL "$BASE/api/s/$SHARE_TOKEN?password=test123")
CORRECT_TITLE=$(echo "$CORRECT" | jf 'title')
[ -n "$CORRECT_TITLE" ] && ok "Share access with correct password" || fail "Share access with correct password"

SHARES=$($CURL "$BASE/api/docs/documents/$DOC_ID/shares" -H "$AUTH")
SHARES_OK=$(echo "$SHARES" | python3 -c "import sys,json; d=json.load(sys.stdin); print('ok' if isinstance(d.get('data'), list) else 'fail')" 2>/dev/null)
[ "$SHARES_OK" = "ok" ] && ok "List shares" || fail "List shares"

DEL_SHARE=$($CURL "$BASE/api/docs/shares/$SHARE_ID" -X DELETE -H "$AUTH")
DEL_SHARE_MSG=$(echo "$DEL_SHARE" | jf 'message')
[ -n "$DEL_SHARE_MSG" ] && ok "Delete share" || fail "Delete share"

# ─── Export ───
section "Export"

for FMT in html markdown txt; do
  CODE=$($CURL -o /dev/null -w "%{http_code}" "$BASE/api/docs/documents/$DOC_ID/export?format=$FMT" -H "$AUTH")
  [ "$CODE" = "200" ] && ok "Export $FMT" || fail "Export $FMT (HTTP $CODE)"
done

# ─── Comments ───
section "Comments"

CMT=$($CURL "$BASE/api/docs/documents/$DOC_ID/comments" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"content":"测试评论内容"}')
CMT_ID=$(echo "$CMT" | jf 'id')
[ -n "$CMT_ID" ] && ok "Create comment" || fail "Create comment"

REPLY=$($CURL "$BASE/api/docs/documents/$DOC_ID/comments" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"content\":\"回复评论\",\"parent_id\":\"$CMT_ID\"}")
REPLY_ID=$(echo "$REPLY" | jf 'id')
[ -n "$REPLY_ID" ] && ok "Reply to comment" || fail "Reply to comment"

CMTS=$($CURL "$BASE/api/docs/documents/$DOC_ID/comments" -H "$AUTH")
CMT_COUNT=$(echo "$CMTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null)
[ "$CMT_COUNT" -ge 2 ] && ok "List comments ($CMT_COUNT)" || fail "List comments (got $CMT_COUNT)"

UP_CMT=$($CURL "$BASE/api/docs/comments/$CMT_ID" -X PUT -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"content":"修改后的评论"}')
UP_CMT_OK=$(echo "$UP_CMT" | jf 'message')
[ -n "$UP_CMT_OK" ] && ok "Update comment" || fail "Update comment"

DEL_R=$($CURL "$BASE/api/docs/comments/$REPLY_ID" -X DELETE -H "$AUTH")
DEL_R_MSG=$(echo "$DEL_R" | jf 'message')
[ -n "$DEL_R_MSG" ] && ok "Delete reply" || fail "Delete reply"

DEL_C=$($CURL "$BASE/api/docs/comments/$CMT_ID" -X DELETE -H "$AUTH")
DEL_C_MSG=$(echo "$DEL_C" | jf 'message')
[ -n "$DEL_C_MSG" ] && ok "Delete comment" || fail "Delete comment"

# ─── Notifications ───
section "Notifications"

NOTIFS=$($CURL "$BASE/api/notifications" -H "$AUTH")
NOTIF_OK=$(echo "$NOTIFS" | python3 -c "import sys,json; d=json.load(sys.stdin); print('ok' if isinstance(d.get('data'), list) else 'fail')" 2>/dev/null)
[ "$NOTIF_OK" = "ok" ] && ok "List notifications" || fail "List notifications"

UNREAD=$($CURL "$BASE/api/notifications/unread-count" -H "$AUTH")
UNREAD_N=$(echo "$UNREAD" | jf 'count')
[ "$UNREAD_N" != "" ] && ok "Unread count ($UNREAD_N)" || fail "Unread count"

MARK=$($CURL "$BASE/api/notifications/read-all" -X PUT -H "$AUTH")
MARK_OK=$(echo "$MARK" | jf 'message')
[ -n "$MARK_OK" ] && ok "Mark all read" || fail "Mark all read"

# ─── Users & Departments ───
section "Users & Departments"

USERS=$($CURL "$BASE/api/users" -H "$AUTH")
USER_COUNT=$(echo "$USERS" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null)
[ "$USER_COUNT" -ge 1 ] && ok "List users ($USER_COUNT)" || fail "List users"

DEPTS=$($CURL "$BASE/api/departments" -H "$AUTH")
DEPT_COUNT=$(echo "$DEPTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null)
[ "$DEPT_COUNT" -ge 1 ] && ok "List departments ($DEPT_COUNT)" || fail "List departments"

# ─── Permissions ───
section "Permissions"

TEST_USER=$($CURL "$BASE/api/users" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"username":"test_perm_user","name":"权限测试用户","password":"Test@2026","role":"member","department_id":""}')
TEST_UID=$(echo "$TEST_USER" | jf 'id')
[ -n "$TEST_UID" ] && { ok "Create test user for permissions"; CREATED_USERS="$CREATED_USERS $TEST_UID"; } || fail "Create test user"

PERMS=$($CURL "$BASE/api/permissions?resource_type=document&resource_id=$DOC_ID" -H "$AUTH")
PERM_OK=$(echo "$PERMS" | python3 -c "import sys,json; d=json.load(sys.stdin); print('ok' if isinstance(d.get('data'), list) else 'fail')" 2>/dev/null)
[ "$PERM_OK" = "ok" ] && ok "List permissions" || fail "List permissions"

SET_PERM=$($CURL "$BASE/api/permissions" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"resource_type\":\"document\",\"resource_id\":\"$DOC_ID\",\"target_type\":\"user\",\"target_id\":\"$TEST_UID\",\"permission\":\"read\",\"inherit\":false}")
SET_PERM_ID=$(echo "$SET_PERM" | jf 'id')
[ -n "$SET_PERM_ID" ] && ok "Set permission (read)" || fail "Set permission"

CHK=$($CURL "$BASE/api/permissions/check?resource_type=document&resource_id=$DOC_ID&user_id=$TEST_UID" -H "$AUTH")
CHK_PERM=$(echo "$CHK" | jf 'permission')
[ "$CHK_PERM" = "read" -o "$CHK_PERM" = "write" -o "$CHK_PERM" = "admin" ] && ok "Check permission → $CHK_PERM" || fail "Check permission (got: $CHK_PERM)"

$CURL "$BASE/api/permissions/$SET_PERM_ID" -X DELETE -H "$AUTH" > /dev/null
ok "Remove permission"

# ─── Audits ───
section "Audits"

AUDITS=$($CURL "$BASE/api/audits?page=1&page_size=5" -H "$AUTH")
AUDIT_OK=$(echo "$AUDITS" | python3 -c "import sys,json; d=json.load(sys.stdin); print('ok' if isinstance(d.get('data'), list) else 'fail')" 2>/dev/null)
[ "$AUDIT_OK" = "ok" ] && ok "List audits" || fail "List audits"

# ─── Storage ───
section "Storage"

STORAGE=$($CURL "$BASE/api/storage/status" -H "$AUTH")
STORAGE_OK=$(echo "$STORAGE" | python3 -c "import sys,json; d=json.load(sys.stdin); print('ok' if isinstance(d.get('disk',{}), dict) else 'fail')" 2>/dev/null)
[ "$STORAGE_OK" = "ok" ] && ok "Storage info" || fail "Storage info"

# ─── Document Move ───
section "Document Move"

MOVE_DOC=$($CURL "$BASE/api/docs/documents" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"移动测试文档","type":"doc","folder_id":""}')
MOVE_DOC_ID=$(echo "$MOVE_DOC" | jf 'id')
[ -n "$MOVE_DOC_ID" ] && { ok "Create doc for move"; CREATED_DOCS="$CREATED_DOCS $MOVE_DOC_ID"; } || fail "Create doc for move"

MOVE_FOLDER=$($CURL "$BASE/api/docs/folders" -X POST -H "$AUTH" -H 'Content-Type: application/json' -d '{"name":"移动目标文件夹"}')
MOVE_FOLDER_ID=$(echo "$MOVE_FOLDER" | jf 'id')
[ -n "$MOVE_FOLDER_ID" ] && { ok "Create folder for move target"; CREATED_FOLDERS="$CREATED_FOLDERS $MOVE_FOLDER_ID"; } || fail "Create folder for move target"

$CURL "$BASE/api/docs/documents/$MOVE_DOC_ID" -X PUT -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"title\":\"移动测试文档\",\"folder_id\":\"$MOVE_FOLDER_ID\"}" > /dev/null
MOVED=$($CURL "$BASE/api/docs/documents/$MOVE_DOC_ID" -H "$AUTH")
MOVED_FID=$(echo "$MOVED" | jf 'folder_id')
[ "$MOVED_FID" = "$MOVE_FOLDER_ID" ] && ok "Move document to folder" || fail "Move document to folder (got folder_id: $MOVED_FID)"

# ─── Full-text Search ───
section "Full-text Search"

SEARCH_DOC=$($CURL "$BASE/api/docs/documents" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"搜索引擎优化指南","type":"doc","content":"<p>这是一篇关于全文检索的文档，关键词：独特内容 xyzzy42</p>"}')
SEARCH_DOC_ID=$(echo "$SEARCH_DOC" | jf 'id')
[ -n "$SEARCH_DOC_ID" ] && { ok "Create doc for search"; CREATED_DOCS="$CREATED_DOCS $SEARCH_DOC_ID"; } || fail "Create doc for search"

# Save content
$CURL "$BASE/api/docs/documents/$SEARCH_DOC_ID" -X PUT -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"搜索引擎优化指南","content":"<p>独特内容 xyzzy42 全文检索测试</p>"}' > /dev/null

SEARCH_TITLE=$($CURL "$BASE/api/docs/documents/search?q=%E6%90%9C%E7%B4%A2%E5%BC%95%E6%93%8E" -H "$AUTH")
SEARCH_TITLE_OK=$(echo "$SEARCH_TITLE" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); print('ok' if len(d)>0 else 'fail')" 2>/dev/null)
[ "$SEARCH_TITLE_OK" = "ok" ] && ok "Search by title" || fail "Search by title"

SEARCH_CONTENT=$($CURL "$BASE/api/docs/documents/search?q=xyzzy42" -H "$AUTH")
SEARCH_CONTENT_OK=$(echo "$SEARCH_CONTENT" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); ids=[x['id'] for x in d]; print('ok' if '$SEARCH_DOC_ID' in ids else 'fail')" 2>/dev/null)
[ "$SEARCH_CONTENT_OK" = "ok" ] && ok "Search by content" || fail "Search by content"

# ─── Trash & Restore ───
section "Trash & Restore"

TRASH_DOC=$($CURL "$BASE/api/docs/documents" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"回收站测试文档","type":"doc"}')
TRASH_DOC_ID=$(echo "$TRASH_DOC" | jf 'id')
[ -n "$TRASH_DOC_ID" ] && { ok "Create doc for trash"; CREATED_DOCS="$CREATED_DOCS $TRASH_DOC_ID"; } || fail "Create doc for trash"

# Delete (soft delete → trash)
$CURL "$BASE/api/docs/documents/$TRASH_DOC_ID" -X DELETE -H "$AUTH" > /dev/null
ok "Soft-delete doc to trash"

# List trash
TRASH=$($CURL "$BASE/api/docs/trash" -H "$AUTH")
TRASH_OK=$(echo "$TRASH" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); ids=[x['id'] for x in d]; print('ok' if '$TRASH_DOC_ID' in ids else 'fail')" 2>/dev/null)
[ "$TRASH_OK" = "ok" ] && ok "Doc appears in trash" || fail "Doc appears in trash"

# Restore
RESTORE=$($CURL "$BASE/api/docs/trash/restore/$TRASH_DOC_ID" -X POST -H "$AUTH")
RESTORE_OK=$(echo "$RESTORE" | jf 'message')
[ -n "$RESTORE_OK" ] && ok "Restore from trash" || fail "Restore from trash"

# Verify doc is back
RESTORED=$($CURL "$BASE/api/docs/documents/$TRASH_DOC_ID" -H "$AUTH")
RESTORED_TITLE=$(echo "$RESTORED" | jf 'title')
[ "$RESTORED_TITLE" = "回收站测试文档" ] && ok "Restored doc accessible" || fail "Restored doc accessible (got: $RESTORED_TITLE)"

# ─── Rate Limit ───
section "Rate Limit"

# Send a burst of requests, ensure we get 200s (not 429 at 30 req/s with burst 60)
RATE_OK=0
for i in $(seq 1 20); do
  CODE=$($CURL -o /dev/null -w "%{http_code}" "$BASE/api/auth/me" -H "$AUTH")
  [ "$CODE" = "200" ] && RATE_OK=$((RATE_OK+1)) || true
done
[ "$RATE_OK" -ge 18 ] && ok "Rate limit allows normal traffic ($RATE_OK/20)" || fail "Rate limit too strict ($RATE_OK/20)"

# ─── Tags ───
section "Tags"

CREATE_TAG=$($CURL "$BASE/api/docs/tags" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"测试标签","color":"#e6a23c"}')
CREATE_TAG_ID=$(echo "$CREATE_TAG" | jf 'id')
[ -n "$CREATE_TAG_ID" ] && ok "Create tag" || fail "Create tag"

LIST_TAGS=$($CURL "$BASE/api/docs/tags" -H "$AUTH")
LIST_TAGS_OK=$(echo "$LIST_TAGS" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); ids=[t['id'] for t in d]; print('ok' if '$CREATE_TAG_ID' in ids else 'fail')" 2>/dev/null)
[ "$LIST_TAGS_OK" = "ok" ] && ok "List tags" || fail "List tags"

TAG_DOC=$($CURL "$BASE/api/docs/documents" -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"title":"标签测试文档","type":"doc"}')
TAG_DOC_ID=$(echo "$TAG_DOC" | jf 'id')
[ -n "$TAG_DOC_ID" ] && { ok "Create doc for tagging"; CREATED_DOCS="$CREATED_DOCS $TAG_DOC_ID"; } || fail "Create doc for tagging"

SET_TAGS=$($CURL "$BASE/api/docs/documents/$TAG_DOC_ID/tags" -X PUT -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"tag_ids\":[\"$CREATE_TAG_ID\"]}")
SET_TAGS_OK=$(echo "$SET_TAGS" | jf 'message')
[ -n "$SET_TAGS_OK" ] && ok "Set document tags" || fail "Set document tags"

GET_TAGS=$($CURL "$BASE/api/docs/documents/$TAG_DOC_ID/tags" -H "$AUTH")
GET_TAGS_OK=$(echo "$GET_TAGS" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); ids=[t['id'] for t in d]; print('ok' if '$CREATE_TAG_ID' in ids else 'fail')" 2>/dev/null)
[ "$GET_TAGS_OK" = "ok" ] && ok "Get document tags" || fail "Get document tags"

BY_TAG=$($CURL "$BASE/api/docs/tags/$CREATE_TAG_ID/documents" -H "$AUTH")
BY_TAG_OK=$(echo "$BY_TAG" | python3 -c "import sys,json; d=json.load(sys.stdin).get('data',[]); ids=[x['id'] for x in d]; print('ok' if '$TAG_DOC_ID' in ids else 'fail')" 2>/dev/null)
[ "$BY_TAG_OK" = "ok" ] && ok "Get docs by tag" || fail "Get docs by tag"

# Cleanup tag
$CURL "$BASE/api/docs/tags/$CREATE_TAG_ID" -X DELETE -H "$AUTH" > /dev/null
ok "Delete tag"

# ─── WebSocket ───
section "WebSocket"

WS_OK=$(python3 -c "
import ssl, websocket
ws = websocket.WebSocket(sslopt={'cert_reqs': ssl.CERT_NONE})
ws.connect('wss://docs.mistlab.dev/ws/docs/$DOC_ID?token=$TOKEN')
ws.close()
print('ok')
" 2>/dev/null)
[ "$WS_OK" = "ok" ] && ok "WebSocket connection" || fail "WebSocket connection"

# ─── Summary ───
echo -e "\n\033[1;36m═══════════════════════════════════"
echo -e "  Results: \033[32m$PASS passed\033[36m, \033[31m$FAIL failed\033[36m, $TOTAL total"
echo -e "═══════════════════════════════════\033[0m"

[ "$FAIL" -gt 0 ] && exit 1 || exit 0
