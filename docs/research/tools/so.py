#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""360 search -> titles+urls+snippets. usage: so.py "q" """
import sys, re, html, urllib.request, urllib.parse, gzip
UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
def get(u):
    r=urllib.request.Request(u);r.add_header("User-Agent",UA)
    r.add_header("Accept-Language","zh-CN,zh;q=0.9");r.add_header("Accept-Encoding","gzip")
    with urllib.request.urlopen(r,timeout=25) as x:
        d=x.read()
        if x.headers.get("Content-Encoding")=="gzip": d=gzip.decompress(d)
        return d.decode("utf-8","ignore")
def strip(s):
    s=re.sub(r'<script.*?</script>',' ',s,flags=re.S|re.I)
    s=re.sub(r'<style.*?</style>',' ',s,flags=re.S|re.I)
    return html.unescape(re.sub(r'\s+',' ',re.sub(r'<[^>]+>','',s))).strip()
q=sys.argv[1]
try:
    h=get("https://www.so.com/s?q=%s"%urllib.parse.quote(q))
except Exception as e:
    print("ERR",e); sys.exit(1)
if "验证码" in h: print("BLOCKED"); sys.exit(2)
for m in re.finditer(r'<li class="res-list[^"]*"(.*?)</li>', h, re.S):
    b=m.group(1)
    a=re.search(r'<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', b, re.S)
    if not a: continue
    print("T:",strip(a.group(2))[:120]); print("U:",html.unescape(a.group(1))[:150])
    print("S:",strip(b)[:260]); print("---")
