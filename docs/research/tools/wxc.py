#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Sogou WeChat search via headless chrome (avoids antispider). usage: wxc.py "query" [page] [outfile]"""
import sys, subprocess, re, html, urllib.parse

def chrome(url):
    ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
    out = subprocess.run(["timeout", "70", "google-chrome", "--headless=new", "--no-sandbox", "--disable-gpu",
                          "--disable-dev-shm-usage", "--virtual-time-budget=12000", "--user-agent=" + ua,
                          "--dump-dom", url], capture_output=True, timeout=90)
    return out.stdout.decode("utf-8", "ignore")

def strip(s):
    s = re.sub(r"<!--.*?-->", "", s, flags=re.S)
    s = re.sub(r"<script.*?</script>", " ", s, flags=re.S)
    return html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", s))).strip()

q = sys.argv[1]
page = sys.argv[2] if len(sys.argv) > 2 else "1"
outf = sys.argv[3] if len(sys.argv) > 3 else ""
url = "https://weixin.sogou.com/weixin?type=2&query=%s&page=%s" % (urllib.parse.quote(q), page)
h = chrome(url)
if outf:
    open(outf, "w").write(h)
print("### WX:", q, "page", page, "len", len(h))
items = re.split(r'<li id="sogou_vr_11002601_box_', h)[1:]
for it in items:
    t = re.search(r'<h3>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', it, re.S)
    s = re.search(r'<p class="txt-info"[^>]*>(.*?)</p>', it, re.S)
    acc = re.search(r'<span class="all-time-y2">(.*?)</span>', it, re.S)
    ts = re.search(r"timeConvert\('(\d+)'\)", it)
    if not t:
        continue
    u = html.unescape(t.group(1)).replace("&amp;", "&")
    if u.startswith("/link"):
        u = "https://weixin.sogou.com" + u
    import time as _t
    date = _t.strftime("%Y-%m-%d", _t.gmtime(int(ts.group(1)))) if ts else "-"
    print("T:", strip(t.group(2)))
    print("A:", strip(acc.group(1)) if acc else "-", "|", date)
    print("U:", u[:280])
    print("S:", strip(s.group(1)) if s else "")
    print("---")
print("count", len(items))
