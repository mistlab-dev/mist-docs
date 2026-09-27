#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Sogou WeChat article search: title + summary + account + date.
usage: wx.py "query" [page]
"""
import sys, re, html, urllib.request, urllib.parse, gzip, time

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def get(url):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", UA)
    r.add_header("Accept-Language", "zh-CN,zh;q=0.9")
    r.add_header("Accept-Encoding", "gzip")
    r.add_header("Referer", "https://weixin.sogou.com/")
    with urllib.request.urlopen(r, timeout=25) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip":
            d = gzip.decompress(d)
        return d.decode("utf-8", "ignore")

def strip(s):
    s = re.sub(r"<!--.*?-->", "", s, flags=re.S)
    s = re.sub(r"<script.*?</script>", " ", s, flags=re.S)
    return html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", s))).strip()

q = sys.argv[1]
page = sys.argv[2] if len(sys.argv) > 2 else "1"
h = get("https://weixin.sogou.com/weixin?type=2&query=%s&page=%s" % (urllib.parse.quote(q), page))
if "antispider" in h or "验证码" in h:
    print("ANTISPIDER")
    sys.exit(0)
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
    date = time.strftime("%Y-%m-%d", time.gmtime(int(ts.group(1)))) if ts else "-"
    print("T:", strip(t.group(2)))
    print("A:", strip(acc.group(1)) if acc else "-", "|", date)
    print("U:", u[:300])
    print("S:", strip(s.group(1)) if s else "")
    print("---")
print("count", len(items))
