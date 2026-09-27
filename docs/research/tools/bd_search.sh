#!/bin/bash
# usage: bd_search.sh <outfile> <query>
out="$1"; q="$2"
Q=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$q")
openclaw browser navigate "https://www.baidu.com/s?wd=$Q&rn=20" >/dev/null 2>&1
sleep 5
{ echo "########## QUERY: $q"; openclaw browser evaluate --fn "() => { const out=[]; document.querySelectorAll('#content_left > div').forEach(d=>{const a=d.querySelector('h3 a'); const t=(d.innerText||'').replace(/\s+/g,' ').trim(); if(t && t.length>40) out.push((a?('TITLE: '+a.innerText.trim()) : 'TITLE: -')+'\n'+t.slice(0,700));}); return out.slice(0,18).join('\n===\n'); }"; } >> "$out" 2>&1
