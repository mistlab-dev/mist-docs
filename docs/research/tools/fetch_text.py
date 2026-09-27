#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Fetch a URL, strip HTML, print text. Usage: fetch_text.py URL [maxchars]"""
import sys, re, html, urllib.request, gzip, urllib.parse

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def get(url, extra=None, ua=UA, timeout=25):
    r = urllib.request.Request(url)
    r.add_header("User-Agent", ua)
    r.add_header("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
    r.add_header("Accept-Encoding", "gzip, deflate")
    for k, v in (extra or {}).items():
        r.add_header(k, v)
    with urllib.request.urlopen(r, timeout=timeout) as resp:
        d = resp.read()
        if resp.headers.get("Content-Encoding") == "gzip":
            d = gzip.decompress(d)
        enc = "utf-8"
        m = re.search(r"charset=[\"']?([\w-]+)", resp.headers.get("Content-Type", "") or "", re.I)
        if m:
            enc = m.group(1)
        return d.decode(enc, "ignore"), resp.geturl()

def text(h):
    h = re.sub(r"<script.*?</script>", " ", h, flags=re.S | re.I)
    h = re.sub(r"<style.*?</style>", " ", h, flags=re.S | re.I)
    h = re.sub(r"<!--.*?-->", " ", h, flags=re.S)
    h = re.sub(r"<(br|/p|/div|/li|/h\d|/tr)[^>]*>", "\n", h, flags=re.I)
    h = re.sub(r"<[^>]+>", " ", h)
    h = html.unescape(h)
    h = re.sub(r"[ \t\u00a0]+", " ", h)
    h = re.sub(r"\n\s*\n+", "\n", h)
    return h.strip()

if __name__ == "__main__":
    url = sys.argv[1]
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else 6000
    try:
        h, final = get(url)
    except Exception as e:
        print("ERR", e)
        sys.exit(1)
    t = text(h)
    print("FINAL_URL:", final)
    print("LEN:", len(t))
    print(t[:limit])
