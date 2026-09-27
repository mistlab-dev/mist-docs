#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Baidu web search extractor. usage: bd.py "query" [pn]"""
import sys, re, html, urllib.request, urllib.parse, gzip

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def get(url):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", UA)
    r.add_header("Accept-Language", "zh-CN,zh;q=0.9")
    r.add_header("Accept-Encoding", "gzip")
    r.add_header("Cookie", "BAIDUID=0:FG=1")
    with urllib.request.urlopen(r, timeout=25) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip": d = gzip.decompress(d)
        return d.decode("utf-8", "ignore")

def strip(s):
    s = re.sub(r'<script.*?</script>', ' ', s, flags=re.S|re.I)
    return html.unescape(re.sub(r'\s+', ' ', re.sub(r'<[^>]+>', '', s))).strip()

q = sys.argv[1]
pn = sys.argv[2] if len(sys.argv) > 2 else "0"
h = get("https://www.baidu.com/s?wd=%s&pn=%s&rn=20" % (urllib.parse.quote(q), pn))
if "安全验证" in h or "wappass" in h:
    print("BLOCKED"); sys.exit(1)
n = 0
for m in re.finditer(r'<div class="result[^"]*"[^>]*>(.*?)(?=<div class="result|<div id="page)', h, re.S):
    b = m.group(0)
    a = re.search(r'<h3[^>]*>.*?<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', b, re.S)
    if not a: continue
    print("T:", strip(a.group(2))[:130])
    print("U:", html.unescape(a.group(1))[:150])
    txt = strip(b)
    print("S:", txt[:300])
    print("---")
    n += 1
print("count", n)
