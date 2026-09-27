#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Toutiao search extractor (so.toutiao.com SSR payload).
usage: tts.py "query" [page]
"""
import sys, re, html, urllib.request, urllib.parse, gzip, json

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def get(url):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", UA)
    r.add_header("Accept-Language", "zh-CN,zh;q=0.9")
    r.add_header("Accept-Encoding", "gzip")
    with urllib.request.urlopen(r, timeout=30) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip":
            d = gzip.decompress(d)
        return d.decode("utf-8", "ignore")

def clean(s):
    s = s.replace("\\u003cem\\u003e", "").replace("\\u003c/em\\u003e", "")
    s = s.replace("<em>", "").replace("</em>", "")
    s = s.replace("\\u002F", "/").replace("\\/", "/")
    s = re.sub(r"\\u([0-9a-fA-F]{4})", lambda m: chr(int(m.group(1), 16)), s)
    s = html.unescape(s)
    return re.sub(r"\s+", " ", s).strip()

q = sys.argv[1]
url = "https://so.toutiao.com/search?keyword=%s&pd=synthesis" % urllib.parse.quote(q)
h = get(url)
# Extract embedded JSON-ish objects containing title+abstract/url
pat = re.compile(r'\\?"title\\?":\\?"(.{4,300}?)\\?",.{0,2000}?\\?"(?:article_url|url|display_url|source_url|share_url)\\?":\\?"(https?://[^"\\]+)')
seen = set()
cnt = 0
for m in pat.finditer(h):
    t = clean(m.group(1))
    if t in seen or len(t) < 6:
        continue
    seen.add(t)
    print("T:", t[:140])
    print("U:", m.group(2)[:220])
    print("---")
    cnt += 1
print("count", cnt)
