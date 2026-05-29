#!/bin/bash
# 前端类型检查脚本 - 专门检查 SheetEditor
set -e

echo "🔍 Checking SheetEditor.vue types..."
cd "$(dirname "$0")/../web"

ERRORS=$(npx vue-tsc --noEmit --skipLibCheck 2>&1 | grep 'SheetEditor' || true)
if [ -n "$ERRORS" ]; then
  echo "❌ SheetEditor has TypeScript errors:"
  echo "$ERRORS"
  exit 1
fi

echo "✅ SheetEditor type check passed"

# 也检查其他组件（不阻塞，只警告）
OTHER=$(npx vue-tsc --noEmit --skipLibCheck 2>&1 | grep -v 'SheetEditor' | grep 'error TS' || true)
if [ -n "$OTHER" ]; then
  echo "⚠️  Other files have TS errors (non-blocking):"
  echo "$OTHER"
fi
