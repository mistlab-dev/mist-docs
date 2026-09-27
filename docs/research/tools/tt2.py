#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Toutiao search results: title, abstract, url, media. usage: tt2.py "query" [rank]"""
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

def dec(s):
    s = re.sub(r"\\u([0-9a-fA-F]{4})", lambda m: chr(int(m.group(1), 16)), s)
    s = s.replace("\\/", "/").replace('\\"', '"')
    s = s.replace("<em>", "").replace("</em>", "")
    return html.unescape(re.sub(r"\s+", " ", s)).strip()

def field(blob, keys):
    for k in keys:
        m = re.search(r'"%s":"(.*?)","' % k, blob, re.S)
        if m:
            return dec(m.group(1))
    return ""

q = sys.argv[1]
url = "https://so.toutiao.com/search?keyword=%s&pd=synthesis" % urllib.parse.quote(q)
h = get(url)
# split candidate item blocks: use "title" ... "group_id"/"item_id"
items = []
for m in re.finditer(r'"title":"(.{6,300}?)"(.{0,12000}?)(?="title":"|$)', h, re.S):
    title = dec(m.group(1))
    blob = m.group(2)
    if len(title) < 6 or title.startswith(","):
        continue
    src = field(blob, ["open_url", "source_url", "seo_url", "display_url", "url"])
    abstract = field(blob, ["abstract", "summary", "content", "description"])
    media = field(blob, ["media_name", "source", "author_name"])
    if not src and not abstract:
        continue
    items.append((title, src, abstract, media))
seen = set()
cnt = 0
for title, src, abstract, media in items:
    if title in seen:
        continue
    seen.add(title)
    print("T:", title[:160])
    print("M:", media[:40], "| U:", src[:160])
    if abstract:
        print("A:", abstract[:400])
    print("---")
    cnt += 1
print("count", cnt)
