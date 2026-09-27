#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Multi-engine search + result extraction for research."""
import sys, re, json, urllib.parse, urllib.request, html, gzip, io

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def fetch(url, extra=None):
    req = urllib.request.Request(url)
    req.add_header("User-Agent", UA)
    req.add_header("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
    req.add_header("Accept-Encoding", "gzip, deflate")
    if extra:
        for k, v in extra.items():
            req.add_header(k, v)
    with urllib.request.urlopen(req, timeout=25) as r:
        data = r.read()
        if r.headers.get("Content-Encoding") == "gzip":
            data = gzip.decompress(data)
        enc = "utf-8"
        ctype = r.headers.get("Content-Type", "")
        m = re.search(r"charset=([\w-]+)", ctype, re.I)
        if m:
            enc = m.group(1)
        try:
            return data.decode(enc, "ignore")
        except Exception:
            return data.decode("utf-8", "ignore")

def strip_tags(s):
    s = re.sub(r"<script.*?</script>", " ", s, flags=re.S | re.I)
    s = re.sub(r"<style.*?</style>", " ", s, flags=re.S | re.I)
    s = re.sub(r"<[^>]+>", "", s)
    return html.unescape(re.sub(r"\s+", " ", s)).strip()

def search_bing(q, count=15):
    url = "https://cn.bing.com/search?q=%s&count=%d&mkt=zh-CN&ensearch=0" % (urllib.parse.quote(q), count)
    h = fetch(url, {"Cookie": "SRCHHPGUSR=SRCHLANG=zh-Hans"})
    out = []
    for m in re.finditer(r'<li class="b_algo".*?</li>', h, re.S):
        blk = m.group(0)
        t = re.search(r'<h2[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', blk, re.S)
        if not t:
            continue
        sn = re.search(r'<p[^>]*>(.*?)</p>', blk, re.S)
        out.append({"title": strip_tags(t.group(2)), "url": html.unescape(t.group(1)),
                    "snippet": strip_tags(sn.group(1)) if sn else ""})
    return out

def search_so360(q, count=15):
    url = "https://www.so.com/s?q=%s&pn=1" % urllib.parse.quote(q)
    h = fetch(url)
    out = []
    for m in re.finditer(r'<h3\s+class="res-title".*?</h3>(.*?)(?=<h3|</li>|$)', h, re.S):
        blk = m.group(0)
        t = re.search(r'data-mdurl="([^"]+)"', blk)
        title = re.search(r'<a[^>]*>(.*?)</a>', blk, re.S)
        if not t:
            continue
        out.append({"title": strip_tags(title.group(1)) if title else "", "url": html.unescape(t.group(1)),
                    "snippet": strip_tags(blk)[:300]})
    return out

def search_sogou(q, count=15):
    url = "https://www.sogou.com/web?query=%s" % urllib.parse.quote(q)
    h = fetch(url)
    out = []
    for m in re.finditer(r'<h3[^>]*>(.*?)</h3>', h, re.S):
        blk = m.group(0)
        a = re.search(r'href="([^"]+)"', blk)
        if not a:
            continue
        u = html.unescape(a.group(1))
        if u.startswith("/link"):
            u = "https://www.sogou.com" + u
        out.append({"title": strip_tags(blk), "url": u, "snippet": ""})
    return out

def main():
    engine = sys.argv[1]
    q = sys.argv[2]
    fn = {"bing": search_bing, "so": search_so360, "sogou": search_sogou}[engine]
    try:
        res = fn(q)
    except Exception as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False))
        return
    print(json.dumps(res, ensure_ascii=False, indent=1))

if __name__ == "__main__":
    main()
