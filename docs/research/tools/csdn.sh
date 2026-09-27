#!/bin/bash
# csdn.sh "query" outname  -- CSDN search via headless chrome
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
Q=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$1")
timeout 70 google-chrome --headless=new --no-sandbox --disable-gpu --disable-dev-shm-usage \
  --virtual-time-budget=15000 --user-agent="$UA" --dump-dom \
  "https://so.csdn.net/so/search?q=$Q&t=blog" 2>/dev/null > "/tmp/csdn_$2.html"
python3 - "$1" "/tmp/csdn_$2.html" <<'PY'
import sys,re,html
q,f=sys.argv[1],sys.argv[2]
t=open(f,errors='ignore').read()
t=re.sub(r'<script.*?</script>',' ',t,flags=re.S); t=re.sub(r'<style.*?</style>',' ',t,flags=re.S)
# extract article links + titles from DOM
links=re.findall(r'href="(https://blog\.csdn\.net/[^"]+)"[^>]*>(.*?)</a>', t, re.S)
seen=set()
print("### CSDN:", q)
for u,ti in links:
    ti=html.unescape(re.sub(r'\s+',' ',re.sub(r'<[^>]+>','',ti))).strip()
    if len(ti)<8 or u in seen: continue
    seen.add(u)
    print("T:",ti[:140]); print("U:",u[:180])
plain=html.unescape(re.sub(r'\s+',' ',re.sub(r'<[^>]+>',' ',t)))
# print the description blob region
i=plain.find('搜索')
print("PAGE:",plain[:2500])
PY
