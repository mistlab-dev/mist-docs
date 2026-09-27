#!/bin/bash
# usage: s.sh <bing|360|sogou|generic> <query>
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
q=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$2")
case "$1" in
  bing) url="https://cn.bing.com/search?q=$q&count=20";;
  sogou) url="https://www.sogou.com/web?query=$q";;
  360) url="https://www.so.com/s?q=$q";;
esac
tmp=$(mktemp)
curl -sS -m 30 -A "$UA" -H "Accept-Language: zh-CN,zh;q=0.9" -o "$tmp" "$url" || { echo "FETCH_FAIL"; exit 1; }
python3 /root/work/research/tmp/parse.py "$1" < "$tmp"
rm -f "$tmp"
