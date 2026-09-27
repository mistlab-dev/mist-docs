import sys, re, html

p, kw = sys.argv[1], sys.argv[2]
raw = open(p, encoding='utf-8', errors='ignore').read()
t = re.sub(r'<script.*?</script>', '', raw, flags=re.S)
t = re.sub(r'<style.*?</style>', '', t, flags=re.S)
t = re.sub(r'<!--.*?-->', '', t, flags=re.S)
txt = html.unescape(re.sub(r'<[^>]+>', '\n', t))
lines = [l.strip() for l in txt.split('\n')]
lines = [l for l in lines if l]
out = '\n'.join(lines)
kwl = sys.argv[2].split('|')
# print from first occurrence of any keyword
idx = None
for k in kwl:
    i = out.find(k)
    if i >= 0 and (idx is None or i < idx):
        idx = i
if idx is None:
    print('KW_NOT_FOUND')
    print(out[-3000:])
else:
    print(out[idx:idx + int(sys.argv[3] if len(sys.argv) > 3 else 8000)])
