#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Sogou web search extractor with snippets.
usage: sg.py "query" [page] """
import sys, re, html, urllib.request, urllib.parse, gzip, time

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

def strip(s):
    s = re.sub(r"<script.*?</script>", " ", s, flags=re.S | re.I)
    s = re.sub(r"<style.*?</style>", " ", s, flags=re.S | re.I)
    return html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", s))).strip()

q = sys.argv[1]
page = sys.argv[2] if len(sys.argv) > 2 else "1"
h = get("https://www.sogou.com/web?query=%s&page=%s" % (urllib.parse.quote(q), page))
blocks = re.split(r'<div class="vrwrap"|<div class="rb"', h)
n = 0
for b in blocks[1:]:
    a = re.search(r'<h3[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', b, re.S)
    if not a:
        continue
    u = html.unescape(a.group(1))
    if u.startswith("/link"):
        u = "https://www.sogou.com" + u
    title = strip(a.group(2))
    if not title:
        continue
    txt = strip(b)
    # cut off the "反馈" tail
    txt = re.sub(r"^.*?反馈", "", txt, count=1) if "反馈" in txt else txt
    print("T:", title[:120])
    print("U:", u[:200])
    print("S:", txt[:600])
    print("---")
    n += 1
print("count", n)
