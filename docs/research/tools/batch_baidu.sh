#!/bin/bash
# usage: batch_baidu.sh <outdir> <query1> <query2> ...
outdir="$1"; shift
mkdir -p "$outdir"
for q in "$@"; do
  f="$outdir/$(echo "$q"|md5sum|cut -c1-8).txt"
  Q=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$q")
  { echo "########## QUERY: $q"; 
    openclaw browser navigate "https://www.baidu.com/s?wd=$Q&rn=20" >/dev/null 2>&1
    sleep 5
    openclaw browser evaluate --fn "() => { const out=[]; document.querySelectorAll('#content_left > div').forEach(d=>{const a=d.querySelector('h3 a'); const t=(d.innerText||'').replace(/\s+/g,' ').trim(); if(t && t.length>40) out.push((a?('TITLE: '+a.innerText.trim()+'\nHREF: '+a.href) : 'TITLE: -')+'\n'+t.slice(0,800));}); return out.slice(0,18).join('\n===\n'); }"
    echo
  } >> "$f" 2>&1
  echo "OK: $q -> $f"
  sleep 3
done
