#!/bin/bash
# usage: art2.sh <url> <start-kw> [len]
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
tmp=$(mktemp)
curl -sS -m 30 -A "$UA" -H "Accept-Language: zh-CN,zh;q=0.9" -L -o "$tmp" "$1"
echo "### $1 bytes=$(wc -c < "$tmp")"
python3 - "$tmp" "$2" "${3:-8000}" <<'PY'
import sys,re,html
raw=open(sys.argv[1],encoding='utf-8',errors='ignore').read()
t=re.sub(r'<script.*?</script>','',raw,flags=re.S)
t=re.sub(r'<style.*?</style>','',t,flags=re.S)
t=re.sub(r'<!--.*?-->','',t,flags=re.S)
txt=html.unescape(re.sub(r'<[^>]+>','\n',t))
lines=[l.strip() for l in txt.split('\n') if l.strip()]
out='\n'.join(lines)
kw=sys.argv[2].split('|')
idx=None
for k in kw:
    i=out.find(k)
    if i>=0 and (idx is None or i<idx): idx=i
if idx is None:
    print('KW_NOT_FOUND'); print(out[-2500:])
else:
    print(out[max(0,idx-200):idx+int(sys.argv[3])])
PY
rm -f "$tmp"
