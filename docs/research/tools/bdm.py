#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Mobile Baidu search extractor. usage: bdm.py "query" [page]"""
import sys, re, html, urllib.request, urllib.parse, gzip, json

UA = "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"

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

def strip(s):
    s = re.sub(r"<script.*?</script>", " ", s, flags=re.S | re.I)
    s = re.sub(r"<!--.*?-->", " ", s, flags=re.S)
    return html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", s))).strip()

q = sys.argv[1]
pg = sys.argv[2] if len(sys.argv) > 2 else "0"
url = "https://m.baidu.com/s?word=%s&pn=%s" % (urllib.parse.quote(q), pg)
h = get(url)
if "安全验证" in h[:3000]:
    print("CAPTCHA"); sys.exit(0)
n = 0
# results appear as <div class="result c-container ..." ...> with data-log
for m in re.finditer(r'<div[^>]*class="[^"]*result[^"]*c-container[^"]*"[^>]*>(.*?)(?=<div[^>]*class="[^"]*result[^"]*c-container|$)', h, re.S):
    b = m.group(0)
    t = re.search(r'<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', b, re.S)
    title = ""
    for mm in re.finditer(r'<a[^>]*href="(http[^"]+)"[^>]*>(.*?)</a>', b, re.S):
        cand = strip(mm.group(2))
        if len(cand) > 8:
            t = mm; title = cand; break
    if not t or not title:
        continue
    print("T:", title[:130])
    print("U:", html.unescape(t.group(1))[:200])
    body = strip(b)
    print("S:", body[:400])
    print("---")
    n += 1
print("count", n)
