#!/bin/bash
# usage: wxresolve.sh <sogou-link-path>
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
u="https://weixin.sogou.com$1"
curl -sS -m 25 -A "$UA" -b /tmp/wxck.txt -c /tmp/wxck.txt -L -o /tmp/wxr.html -w "code=%{http_code} url=%{url_effective}\n" "$u"
grep -oE 'url \+= .[^;]+' /tmp/wxr.html | head -20
python3 - <<'PY'
import re
t=open('/tmp/wxr.html',encoding='utf-8',errors='ignore').read()
m=re.findall(r"url \+= '([^']*)'",t)
print('PARTS:',len(m))
print(''.join(m)[:300])
i=t.find('var url')
print(t[i:i+600] if i>0 else 'no var url')
PY
