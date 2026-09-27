#!/bin/bash
# usage: art.sh <url> <kw-regex-alternation> [len]
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
tmp=$(mktemp)
curl -sS -m 30 -A "$UA" -H "Accept-Language: zh-CN,zh;q=0.9" -L -o "$tmp" "$1"
echo "### $1  bytes=$(wc -c < "$tmp")"
python3 /root/work/research/tmp/body.py "$tmp" "$2" "${3:-8000}"
rm -f "$tmp"
