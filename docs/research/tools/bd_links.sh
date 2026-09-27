#!/bin/bash
# usage: bd_links.sh <query>  -> prints TITLE|HREF for content results
Q=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$1")
openclaw browser navigate "https://www.baidu.com/s?wd=$Q&rn=20" >/dev/null 2>&1
sleep 5
openclaw browser evaluate --fn "() => { const out=[]; document.querySelectorAll('#content_left > div').forEach(d=>{const a=d.querySelector('h3 a'); if(!a) return; const t=a.innerText.trim(); if(t.length<6) return; out.push(t+' ||| '+a.href);}); return out.join('\n'); }"
