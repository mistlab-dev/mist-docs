import sys, re, html

eng = sys.argv[1]
t = sys.stdin.read()
t = re.sub(r'<script.*?</script>', '', t, flags=re.S)
t = re.sub(r'<style.*?</style>', '', t, flags=re.S)

if eng == 'bing':
    items = re.findall(r'<li class="b_algo".*?</li>', t, flags=re.S)
    for it in items:
        m = re.search(r'<h2>.*?<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', it, flags=re.S)
        if m:
            title = html.unescape(re.sub(r'<[^>]+>', '', m.group(2)))
            sn = re.search(r'<p[^>]*>(.*?)</p>', it, flags=re.S)
            snip = html.unescape(re.sub(r'<[^>]+>', '', sn.group(1))) if sn else ''
            print(f"URL: {m.group(1)}\nT: {title.strip()}\nS: {snip.strip()[:400]}\n")
elif eng == '360':
    items = re.findall(r'<li class="res-list".*?</li>', t, flags=re.S)
    for it in items:
        m = re.search(r'<h3[^>]*>.*?<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', it, flags=re.S)
        if m:
            title = html.unescape(re.sub(r'<[^>]+>', '', m.group(2)))
            sn = re.search(r'<p[^>]*>(.*?)</p>', it, flags=re.S)
            snip = html.unescape(re.sub(r'<[^>]+>', '', sn.group(1))) if sn else ''
            print(f"URL: {m.group(1)}\nT: {title.strip()}\nS: {snip.strip()[:400]}\n")
elif eng == 'sogou':
    items = re.findall(r'<div class="vrwrap".*?</div>\s*</div>', t, flags=re.S)
    for it in items:
        m = re.search(r'<h3[^>]*>.*?<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', it, flags=re.S)
        if m:
            title = html.unescape(re.sub(r'<[^>]+>', '', m.group(2)))
            print(f"URL: {m.group(1)}\nT: {title.strip()}\n")
else:
    links = re.findall(r'href="(https?://[^"]+)"[^>]*>(.*?)</a>', t, flags=re.S)
    seen = set()
    for u, txt in links:
        txt = html.unescape(re.sub(r'<[^>]+>', '', txt)).strip()
        if len(txt) < 8 or u in seen:
            continue
        if any(x in u for x in ['sogou', 'so.com', 'bing', 'baidu.com', '/link?', 'javascript', '360.cn']):
            continue
        seen.add(u)
        print(f"URL: {u}\nT: {txt[:200]}\n")
