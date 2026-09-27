#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Resolve a sogou weixin link (from wx.py output) into the real mp.weixin.qq.com URL and dump text.
usage: wxart.py <sogou-link> [maxchars]
"""
import sys, re, html, urllib.request, urllib.parse, gzip, time

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def get(url, ref=None):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", UA)
    r.add_header("Accept-Language", "zh-CN,zh;q=0.9")
    r.add_header("Accept-Encoding", "gzip")
    if ref: r.add_header("Referer", ref)
    with urllib.request.urlopen(r, timeout=30) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip": d = gzip.decompress(d)
        return d.decode("utf-8", "ignore")

def strip(s):
    s = re.sub(r'<script.*?</script>', ' ', s, flags=re.S|re.I)
    s = re.sub(r'<style.*?</style>', ' ', s, flags=re.S|re.I)
    s = re.sub(r'<!--.*?-->', ' ', s, flags=re.S)
    t = html.unescape(re.sub(r'<[^>]+>', '\n', s))
    lines = [l.strip() for l in t.split('\n')]
    return '\n'.join([l for l in lines if l])

link = sys.argv[1]
mc = int(sys.argv[2]) if len(sys.argv) > 2 else 8000
h = get(link, "https://weixin.sogou.com/")
parts = re.findall(r"url \+= '([^']*)'", h)
real = "".join(parts).replace("@", "") if parts else ""
if not real:
    m = re.search(r"(https?://mp\.weixin\.qq\.com/[^\"'\s<>]+)", h)
    real = html.unescape(m.group(1)) if m else ""
if not real:
    print("RESOLVE_FAIL"); print(h[:800]); sys.exit(1)
print("REAL:", real)
try:
    art = get(real, "https://weixin.sogou.com/")
except Exception as e:
    print("FETCH_FAIL", e); sys.exit(2)
m = re.search(r'id="js_content".*?>(.*?)</div>\s*</div>', art, re.S)
body = m.group(1) if m else art
txt = strip(body)
print("---TITLE:", (re.search(r'property="og:title" content="([^"]*)"', art) or [None,''])[1])
print("---LEN:", len(txt))
print(txt[:mc])
