#!/bin/bash
set -e
BASE="https://docs.mistlab.dev"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

check() {
  local name="$1"
  local code="$2"
  local expect="${3:-200}"
  if [ "$code" = "$expect" ]; then
    echo "${GREEN}✅ PASS${NC} $name"
    PASS=$((PASS+1))
  else
    echo "${RED}❌ FAIL${NC} $name (got $code, expect $expect)"
    FAIL=$((FAIL+1))
  fi
}

echo -e "${YELLOW}━━━ Auth ━━━${NC}"
TOKEN=$(curl -sk "$BASE/api/auth/login" -X POST -H 'Content-Type: application/json' -d '{"username":"admin","password":"Admin@2026"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
[ -n "$TOKEN" ] && check "Login" 200 || check "Login" 0

curl -sk "$BASE/api/auth/me" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}'
check "Me" "$(curl -sk "$BASE/api/auth/me" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo -e "${YELLOW}━━━ Documents ━━━${NC}"
DOCID=$(curl -sk "$BASE/api/docs/documents" -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(d[0]['id'] if d else '')")
[ -n "$DOCID" ] && check "List documents" 200 || check "List documents" 0

check "Get content" "$(curl -sk "$BASE/api/docs/documents/$DOCID/content" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "List versions" "$(curl -sk "$BASE/api/docs/documents/$DOCID/versions" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Search" "$(curl -sk "$BASE/api/docs/documents/search?q=test" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Recent" "$(curl -sk "$BASE/api/docs/documents/recent" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Favorites" "$(curl -sk "$BASE/api/docs/favorites" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Trash" "$(curl -sk "$BASE/api/docs/trash" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo -e "${YELLOW}━━━ Share ━━━${NC}"
SHARE=$(curl -sk "$BASE/api/docs/documents/$DOCID/share" -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"password":"test","expires_in":24}')
SHARE_TOKEN=$(echo $SHARE | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
[ -n "$SHARE_TOKEN" ] && check "Create share" 200 || check "Create share" 0

check "Access share (no pwd)" "$(curl -sk "$BASE/api/s/$SHARE_TOKEN" -o /dev/null -w '%{http_code}')" 403
check "Access share (wrong pwd)" "$(curl -sk "$BASE/api/s/$SHARE_TOKEN?password=wrong" -o /dev/null -w '%{http_code}')" 403
check "Access share (correct)" "$(curl -sk "$BASE/api/s/$SHARE_TOKEN?password=test" -o /dev/null -w '%{http_code}')"

SHARE_ID=$(echo $SHARE | python3 -c "import sys,json; print(json.load(sys.stdin)['share_id'])")
curl -sk -X DELETE "$BASE/api/docs/shares/$SHARE_ID" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}'
check "Delete share" "$(curl -sk -X DELETE "$BASE/api/docs/shares/$SHARE_ID" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo -e "${YELLOW}━━━ Export ━━━${NC}"
check "Export HTML" "$(curl -sk "$BASE/api/docs/documents/$DOCID/export?format=html" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Export Markdown" "$(curl -sk "$BASE/api/docs/documents/$DOCID/export?format=markdown" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Export TXT" "$(curl -sk "$BASE/api/docs/documents/$DOCID/export?format=txt" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo -e "${YELLOW}━━━ Comments ━━━${NC}"
C1=$(curl -sk "$BASE/api/docs/documents/$DOCID/comments" -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"content":"测试评论"}')
C1_ID=$(echo $C1 | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
[ -n "$C1_ID" ] && check "Create comment" 200 || check "Create comment" 0

C2=$(curl -sk "$BASE/api/docs/documents/$DOCID/comments" -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"content\":\"回复\",\"parent_id\":\"$C1_ID\"}")
C2_ID=$(echo $C2 | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
[ -n "$C2_ID" ] && check "Reply comment" 200 || check "Reply comment" 0

check "List comments" "$(curl -sk "$BASE/api/docs/documents/$DOCID/comments" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Delete reply" "$(curl -sk -X DELETE "$BASE/api/docs/comments/$C2_ID" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Delete comment" "$(curl -sk -X DELETE "$BASE/api/docs/comments/$C1_ID" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo -e "${YELLOW}━━━ Notifications ━━━${NC}"
check "List notifications" "$(curl -sk "$BASE/api/notifications" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Unread count" "$(curl -sk "$BASE/api/notifications/unread-count" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
curl -sk -X PUT "$BASE/api/notifications/read-all" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}'
check "Mark all read" "$(curl -sk -X PUT "$BASE/api/notifications/read-all" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo -e "${YELLOW}━━━ Admin ━━━${NC}"
check "Dashboard" "$(curl -sk "$BASE/api/admin/dashboard" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "System info" "$(curl -sk "$BASE/api/admin/system-info" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Users list" "$(curl -sk "$BASE/api/users" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Departments list" "$(curl -sk "$BASE/api/departments" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Permissions list" "$(curl -sk "$BASE/api/permissions?resource_type=document&resource_id=$DOCID" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Audits list" "$(curl -sk "$BASE/api/audits" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"
check "Storage status" "$(curl -sk "$BASE/api/storage/status" -H "Authorization: Bearer $TOKEN" -o /dev/null -w '%{http_code}')"

echo ""
echo -e "${YELLOW}━━━ Summary ━━━${NC}"
echo -e "PASS: ${GREEN}$PASS${NC}  FAIL: ${RED}$FAIL${NC}"
[ $FAIL -eq 0 ]
