#!/bin/bash
# usage: links.sh <url> [pattern]
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
tmp=$(mktemp)
curl -sS -m 30 -A "$UA" -H "Accept-Language: zh-CN,zh;q=0.9" -L -o "$tmp" "$1"
python3 - "$tmp" "${2:-}" <<'PY'
import sys,re,html
t=open(sys.argv[1],encoding='utf-8',errors='ignore').read()
pat=sys.argv[2] if len(sys.argv)>2 else ''
seen=set()
for m in re.finditer(r'href="([^"#]+)"[^>]*>(.*?)</a>',t,flags=re.S):
    u=html.unescape(m.group(1)); txt=html.unescape(re.sub(r'<[^>]+>','',m.group(2))).strip()
    key=(u,txt)
    if key in seen: continue
    seen.add(key)
    if pat and pat not in u: continue
    print(u[:160],'|',txt[:90])
PY
rm -f "$tmp"
