#!/usr/bin/env python3
import sys, re, html, urllib.request, urllib.parse, gzip
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
def get(url, extra=None):
    r=urllib.request.Request(url); r.add_header("User-Agent",UA)
    r.add_header("Accept-Language","zh-CN,zh;q=0.9"); r.add_header("Accept-Encoding","gzip")
    for k,v in (extra or {}).items(): r.add_header(k,v)
    with urllib.request.urlopen(r,timeout=25) as resp:
        d=resp.read()
        if resp.headers.get("Content-Encoding")=="gzip": d=gzip.decompress(d)
        return d.decode("utf-8","ignore")
def strip(s):
    s=re.sub(r"<script.*?</script>"," ",s,flags=re.S|re.I)
    return html.unescape(re.sub(r"\s+"," ",re.sub(r"<[^>]+>","",s))).strip()
q=sys.argv[1]
page=sys.argv[2] if len(sys.argv)>2 else "0"
url="https://cn.bing.com/search?q=%s&first=%s&mkt=zh-CN&setlang=zh-CN&FORM=QBLH"%(urllib.parse.quote(q), str(int(page)*10+1))
h=get(url, {"Cookie":"SRCHHPGUSR=SRCHLANG=zh-Hans; _EDGE_S=mkt=zh-cn"})
print("HTML len",len(h))
n=0
for m in re.finditer(r'<li class="b_algo"[^>]*>(.*?)</li>\s*(?=<li|</ol>)', h, re.S):
    b=m.group(0)
    a=re.search(r'<h2[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', b, re.S)
    if not a: continue
    sn=re.search(r'<p[^>]*>(.*?)</p>', b, re.S)
    print("T:", strip(a.group(2))[:120]); print("U:", html.unescape(a.group(1))[:200])
    print("S:", strip(sn.group(1))[:300] if sn else ""); print("---")
    n+=1
print("count",n)
