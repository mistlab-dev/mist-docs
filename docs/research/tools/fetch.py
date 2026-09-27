#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Fetch page with chrome headless fallback-free; usage fetch.py URL [maxchars] [m|d]"""
import sys, subprocess, re, html, os

def chrome(url, mobile=False, budget=15000):
    ua = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1" if mobile \
        else "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
    out = subprocess.run(["timeout", "70", "google-chrome", "--headless=new", "--no-sandbox", "--disable-gpu",
                          "--disable-dev-shm-usage", "--virtual-time-budget=%d" % budget,
                          "--user-agent=" + ua, "--dump-dom", url],
                         capture_output=True, timeout=90)
    return out.stdout.decode("utf-8", "ignore")

def text(h):
    h = re.sub(r"<script.*?</script>", " ", h, flags=re.S | re.I)
    h = re.sub(r"<style.*?</style>", " ", h, flags=re.S | re.I)
    h = re.sub(r"<!--.*?-->", " ", h, flags=re.S)
    h = re.sub(r"<(br|/p|/div|/li|/h\d|/tr)[^>]*>", "\n", h, flags=re.I)
    h = html.unescape(re.sub(r"<[^>]+>", " ", h))
    h = re.sub(r"[ \t\u00a0]+", " ", h)
    return re.sub(r"\n\s*\n+", "\n", h).strip()

if __name__ == "__main__":
    url = sys.argv[1]
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else 6000
    mob = len(sys.argv) > 3 and sys.argv[3] == "m"
    h = chrome(url, mob)
    t = text(h)
    print("LEN:", len(t))
    print(t[:limit])
