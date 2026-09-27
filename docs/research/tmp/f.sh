#!/bin/bash
# usage: f.sh <url> [maxchars]
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
url="$1"
mc="${2:-6000}"
tmp=$(mktemp)
code=$(curl -sS -m 30 -A "$UA" -H "Accept-Language: zh-CN,zh;q=0.9" -L -o "$tmp" -w "%{http_code}" "$url")
echo "### HTTP $code  $url  (bytes: $(wc -c < "$tmp"))"
python3 - "$tmp" "$mc" <<'PY'
import sys,re,html
p,mc=sys.argv[1],int(sys.argv[2])
raw=open(p,encoding='utf-8',errors='ignore').read()
t=re.sub(r'<script.*?</script>','',raw,flags=re.S)
t=re.sub(r'<style.*?</style>','',t,flags=re.S)
t=re.sub(r'<noscript.*?</noscript>','',t,flags=re.S)
t=re.sub(r'<!--.*?-->','',t,flags=re.S)
txt=html.unescape(re.sub(r'<[^>]+>','\n',t))
lines=[l.strip() for l in txt.split('\n')]
lines=[l for l in lines if l]
out='\n'.join(lines)
print(out[:mc])
PY
rm -f "$tmp"
