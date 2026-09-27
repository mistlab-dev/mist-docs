#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Tieba mobile search: JSON API. usage: tb.py "query" [page]"""
import sys, json, urllib.request, urllib.parse, gzip, re, html

UA = "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"

def get(url):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", UA)
    r.add_header("Accept-Encoding", "gzip")
    with urllib.request.urlopen(r, timeout=25) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip":
            d = gzip.decompress(d)
        return d.decode("utf-8", "ignore")

q = sys.argv[1]
page = sys.argv[2] if len(sys.argv) > 2 else "1"
url = "https://tieba.baidu.com/mo/q/search/thread?word=%s&pn=%s" % (urllib.parse.quote(q), page)
raw = get(url)
try:
    d = json.loads(raw)
except Exception:
    print("NOTJSON", raw[:200]); sys.exit(0)
posts = (d.get("data") or {}).get("post_list") or []
for p in posts:
    t = html.unescape(p.get("title") or "")
    c = html.unescape(re.sub(r"\s+", " ", p.get("content") or ""))
    print("T:", t)
    print("C:", c[:400])
    print("F:", p.get("fname"), "| tid:", p.get("tid"), "| time:", p.get("time"))
    print("U: https://tieba.baidu.com/p/%s" % p.get("tid"))
    print("---")
print("count", len(posts))
