import sys, re, html, subprocess

eng = sys.argv[1]
t = sys.stdin.read()
t = re.sub(r'<script.*?</script>', '', t, flags=re.S)
t = re.sub(r'<style.*?</style>', '', t, flags=re.S)
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"


def clean(x):
    return html.unescape(re.sub(r'<[^>]+>', '', x)).strip()


def resolve(u):
    if u.startswith('/link'):
        u = 'https://www.sogou.com' + u
    if 'so.com/link' not in u and 'sogou.com/link' not in u and '/ck/a' not in u:
        return u
    try:
        p = subprocess.run(['curl', '-sS', '-m', '12', '-A', UA, u], capture_output=True, text=True, timeout=20)
        b = p.stdout
        for pat in [r'location\.replace\(\s*["\'](https?://[^"\'>]+)', r'URL=\s*["\']?(https?://[^"\'>\s]+)',
                    r'content="0;\s*url=(https?://[^"\'>]+)']:
            m = re.search(pat, b, flags=re.I)
            if m:
                return m.group(1)
    except Exception:
        pass
    return u


items = []
if eng == 'bing':
    for m in re.finditer(r'<h2[^>]*>(.*?)</h2>', t, flags=re.S):
        a = re.search(r'<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>', m.group(1), flags=re.S)
        if a:
            items.append((a.group(1), clean(a.group(2)), ''))
elif eng in ('sogou', '360'):
    for m in re.finditer(r'<h3[^>]*>(.*?)</h3>(.*?)(?=<h3|</body>)', t, flags=re.S):
        a = re.search(r'href="([^"]+)"[^>]*>(.*?)</a>', m.group(1), flags=re.S)
        if a:
            body = m.group(2)
            sn = None
            for pat in [r'<p[^>]*>(.*?)</p>', r'<div[^>]*class="[^"]*(?:text-layout|fz-mid|space-txt|res-desc|str_info)[^"]*"[^>]*>(.*?)</div>']:
                sn = re.search(pat, body, flags=re.S)
                if sn:
                    break
            items.append((a.group(1), clean(a.group(2)), clean(sn.group(1))[:400] if sn else ''))

seen = set()
for u, title, snip in items:
    ru = resolve(u)
    if ru in seen or not title:
        continue
    seen.add(ru)
    print(f"URL: {ru}\nT: {title[:200]}\nS: {snip}\n")
