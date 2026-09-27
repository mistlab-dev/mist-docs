#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""WeChat article search via sogou (chrome) for multiple queries -> TSV to stdout."""
import subprocess, re, html, urllib.parse, sys, time

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def chrome(url):
    o = subprocess.run(["timeout", "60", "google-chrome", "--headless=new", "--no-sandbox", "--disable-gpu",
                        "--disable-dev-shm-usage", "--virtual-time-budget=10000", "--user-agent=" + UA,
                        "--dump-dom", url], capture_output=True, timeout=80)
    return o.stdout.decode("utf-8", "ignore")

def run(q):
    h = chrome("https://weixin.sogou.com/weixin?type=2&query=%s" % urllib.parse.quote(q))
    rows = []
    for it in re.split(r'<li id="sogou_vr_11002601_box_', h)[1:]:
        t = re.search(r'<h3>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', it, re.S)
        s = re.search(r'<p class="txt-info"[^>]*>(.*?)</p>', it, re.S)
        a = re.search(r'<span class="all-time-y2">(.*?)</span>', it, re.S)
        ts = re.search(r"timeConvert\('(\d+)'\)", it)
        if not t:
            continue
        u = html.unescape(t.group(1)).replace("&amp;", "&")
        if u.startswith("/link"):
            u = "https://weixin.sogou.com" + u
        ti = html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", t.group(2)))).strip()
        sn = html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", s.group(1)))).strip() if s else ""
        acc = html.unescape(re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", a.group(1)))).strip() if a else ""
        date = time.strftime("%Y-%m-%d", time.gmtime(int(ts.group(1)))) if ts else "-"
        rows.append((q, ti, acc, date, sn, u))
    return rows

if __name__ == "__main__":
    qs = sys.argv[1:]
    for q in qs:
        try:
            for r in run(q):
                print("\t".join(r))
        except Exception as e:
            sys.stderr.write("ERR %s %s\n" % (q, e))
