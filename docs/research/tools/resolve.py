#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Resolve sogou-weixin links to real mp.weixin.qq.com URLs and dump article text.
usage: resolve.py links.tsv outdir
Input TSV: query \t title \t account \t date \t snippet \t sogou_url
"""
import sys, os, re, html, subprocess, json, hashlib

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

def chrome(url, budget=12000):
    o = subprocess.run(["timeout", "60", "google-chrome", "--headless=new", "--no-sandbox", "--disable-gpu",
                        "--disable-dev-shm-usage", "--virtual-time-budget=%d" % budget,
                        "--user-agent=" + UA, "--dump-dom", url], capture_output=True, timeout=80)
    return o.stdout.decode("utf-8", "ignore")

def text(h):
    h = re.sub(r"<script.*?</script>", " ", h, flags=re.S | re.I)
    h = re.sub(r"<style.*?</style>", " ", h, flags=re.S | re.I)
    h = re.sub(r"<!--.*?-->", " ", h, flags=re.S)
    h = re.sub(r"<(br|/p|/div|/li|/h\d|/tr)[^>]*>", "\n", h, flags=re.I)
    h = html.unescape(re.sub(r"<[^>]+>", " ", h))
    h = re.sub(r"[ \t\u00a0]+", " ", h)
    return re.sub(r"\n\s*\n+", "\n", h).strip()

links = sys.argv[1]
outdir = sys.argv[2]
os.makedirs(outdir, exist_ok=True)
idx = []
with open(links) as f:
    for i, line in enumerate(f):
        parts = line.rstrip("\n").split("\t")
        if len(parts) < 6:
            continue
        q, title, acc, date, snippet, url = parts[:6]
        h1 = chrome(url)
        real = ""
        m = re.search(r'var msg_link = "([^"]+)"', h1) or re.search(r"url \+= '([^']+)'", h1)
        if m:
            real = m.group(1)
        # even without resolution, dump the redirected page text
        t = text(h1)
        key = "%02d" % i
        fn = os.path.join(outdir, "%s.md" % key)
        with open(fn, "w") as g:
            g.write("# %s\n\n- query: %s\n- account: %s\n- date: %s\n- link: %s\n- snippet: %s\n\n---\n\n%s\n" %
                    (title, q, acc, date, url, snippet, t))
        idx.append({"id": key, "title": title, "account": acc, "date": date,
                    "snippet": snippet, "sogou": url, "real": real, "len": len(t), "file": fn})
        print(key, len(t), "|", title[:60], "|", real[:80])
with open(os.path.join(outdir, "index.json"), "w") as g:
    json.dump(idx, g, ensure_ascii=False, indent=1)
