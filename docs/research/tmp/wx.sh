#!/bin/bash
# usage: wx.sh <query>
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
q=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$1")
tmp=$(mktemp)
curl -sS -m 30 -A "$UA" -H "Accept-Language: zh-CN,zh;q=0.9" -c /tmp/wxck.txt -o "$tmp" "https://weixin.sogou.com/weixin?type=2&query=$q"
python3 - "$tmp" <<'PY'
import sys,re,html
t=open(sys.argv[1],encoding='utf-8',errors='ignore').read()
t=re.sub(r'<script.*?</script>','',t,flags=re.S)
n=0
for m in re.finditer(r'<h3[^>]*>(.*?)</h3>(.*?)(?=<h3|</body>)',t,flags=re.S):
    a=re.search(r'href="([^"]+)"[^>]*>(.*?)</a>',m.group(1),flags=re.S)
    body=m.group(2)
    sn=re.search(r'<p[^>]*class="txt-info"[^>]*>(.*?)</p>',body,flags=re.S) or re.search(r'<p[^>]*>(.*?)</p>',body,flags=re.S)
    if a:
        n+=1
        print("URL:",html.unescape(a.group(1)))
        print("T:",html.unescape(re.sub(r'<[^>]+>','',a.group(2))).strip()[:160])
        if sn: print("S:",html.unescape(re.sub(r'<[^>]+>','',sn.group(1))).strip()[:350])
        print()
print("n=",n)
PY
rm -f "$tmp"
