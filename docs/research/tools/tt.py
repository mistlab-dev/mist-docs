#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Toutiao search extractor: python3 tt.py "query" """
import sys, urllib.request, urllib.parse, gzip, re, html, json

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def get(url):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", UA)
    r.add_header("Accept-Language", "zh-CN,zh;q=0.9")
    r.add_header("Accept-Encoding", "gzip")
    with urllib.request.urlopen(r, timeout=25) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip":
            d = gzip.decompress(d)
        return d.decode("utf-8", "ignore")

def clean(s):
    s = re.sub(r"\\u003cem\\u003e|\\u003c/em\\u003e|<em>|</em>", "", s)
    s = s.replace("\\u002F", "/").replace('\\"', '"')
    return html.unescape(re.sub(r"\s+", " ", s)).strip()

q = sys.argv[1]
h = get("https://so.toutiao.com/search?keyword=%s&pd=synthesis" % urllib.parse.quote(q))
# Items often have "title" and "article_url"/"url"
items = []
for m in re.finditer(r'\{[^{}]{0,4000}?\\?"title\\?":\s*\\?"(.{4,300}?)\\?"[^{}]{0,3000}?\\?"(?:article_url|url|display_url)\\?":\s*\\?"(https?[^"\\]+)', h):
    items.append((clean(m.group(1)), m.group(2)))
seen = set()
for t, u in items:
    if t in seen:
        continue
    seen.add(t)
    print("T:", t)
    print("U:", u[:220])
    print("-")
print("total items", len(items))
