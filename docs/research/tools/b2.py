#!/usr/bin/env python3
import sys, re, html, urllib.request, urllib.parse, gzip
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
def get(url, extra=None):
    r=urllib.request.Request(url); r.add_header("User-Agent",UA)
    r.add_header("Accept-Language","zh-CN,zh;q=0.9,en;q=0.8"); r.add_header("Accept-Encoding","gzip")
    for k,v in (extra or {}).items(): r.add_header(k,v)
    with urllib.request.urlopen(r,timeout=25) as resp:
        d=resp.read()
        if resp.headers.get("Content-Encoding")=="gzip": d=gzip.decompress(d)
        return d.decode("utf-8","ignore")
def strip(s):
    return html.unescape(re.sub(r"\s+"," ",re.sub(r"<[^>]+>","",s))).strip()
q=sys.argv[1]; first=sys.argv[2] if len(sys.argv)>2 else "1"
for host in ["www.bing.com","cn.bing.com"]:
    url="https://%s/search?q=%s&first=%s&count=20"%(host, urllib.parse.quote(q), first)
    try:
        h=get(url, {"Cookie":"SRCHHPGUSR=SRCHLANG=zh-Hans"})
    except Exception as e:
        print("ERR",host,e); continue
    n=0
    print("=== host",host,"len",len(h))
    for m in re.finditer(r'<li class="b_algo"[^>]*>(.*?)</li>', h, re.S):
        b=m.group(0)
        a=re.search(r'<h2[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', b, re.S)
        if not a: continue
        sn=re.search(r'<p[^>]*>(.*?)</p>', b, re.S)
        print("T:", strip(a.group(2))[:120]); print("U:", html.unescape(a.group(1))[:180])
        print("S:", strip(sn.group(1))[:280] if sn else ""); print("---")
        n+=1
    print("count",n)
