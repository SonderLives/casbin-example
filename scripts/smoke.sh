#!/usr/bin/env bash
# 手动冒烟脚本：先启动后端（cd backend && go run .），再执行 bash scripts/smoke.sh
# 自动化的授权行为验证在 backend/main_test.go —— 推荐：cd backend && go test -v
set -euo pipefail
BASE=${BASE:-http://localhost:8080}

token() {
  curl -s -X POST "$BASE/api/login" -H 'Content-Type: application/json' \
    -d "{\"username\":\"$1\",\"password\":\"123456\"}" \
    | grep -oE '"token":"[^"]+"' | cut -d'"' -f4
}

ALICE=$(token alice); BOB=$(token bob); EVE=$(token eve)
echo "alice token: ${ALICE:0:24}..."
echo

hit() { # METHOD PATH TOKEN [BODY]
  local m=$1 p=$2 t=$3 b=${4:-}
  if [ -n "$b" ]; then
    curl -s -o /dev/null -w '%{http_code}' -X "$m" "$BASE$p" \
      -H "Authorization: Bearer $t" -H 'Content-Type: application/json' -d "$b"
  else
    curl -s -o /dev/null -w '%{http_code}' -X "$m" "$BASE$p" -H "Authorization: Bearer $t"
  fi
}

echo "alice 读 tenant-a 文档      → $(hit GET  /api/domains/tenant-a/docs "$ALICE")  (expect 200, admin)"
echo "alice 在 tenant-b 建文档    → $(hit POST /api/domains/tenant-b/docs "$ALICE" '{"title":"x","content":"y"}')  (expect 403, 在 b 只是访客)"
echo "eve   读 tenant-a 文档      → $(hit GET  /api/domains/tenant-a/docs "$EVE")  (expect 403, 无角色)"
echo "eve   读自己的 doc-5        → $(hit GET  /api/domains/tenant-b/docs/doc-5 "$EVE")  (expect 200, ABAC 属主)"
echo "bob   删别人的 doc-1        → $(hit DELETE /api/domains/tenant-a/docs/doc-1 "$BOB")  (expect 403, editor 无 delete)"
echo "alice 读 tenant-a 策略      → $(hit GET  '/api/policies?dom=tenant-a' "$ALICE")  (expect 200, admin)"
echo "alice 读 tenant-b 策略      → $(hit GET  '/api/policies?dom=tenant-b' "$ALICE")  (expect 403, 域隔离)"
