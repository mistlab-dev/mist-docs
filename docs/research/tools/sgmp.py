#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Sogou web search -> list mp.weixin.qq.com direct links + titles. usage: sgmp.py "query" [page]"""
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

q=sys.argv[1]; page=sys.argv[2] if len(sys.argv)>2 else "1"
h=get("https://www.sogou.com/web?query=%s&page=%s"%(urllib.parse.quote(q),page))
if "验证码" in h: print("BLOCKED"); sys.exit(1)
n=0
for m in re.finditer(r'<h3[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>(.*?)(?=<h3|</div>\s*$)', h, re.S):
    u=html.unescape(m.group(1)); t=strip(m.group(2)); rest=strip(m.group(3))
    if "mp.weixin.qq.com" in u:
        print("T:",t[:120]); print("U:",u[:400]); print("S:",rest[:260]); print("---"); n+=1
print("mp-links:",n)
