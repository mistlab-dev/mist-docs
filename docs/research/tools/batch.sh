#!/bin/bash
# usage: batch.sh engine "query"
e="$1"; q="$2"
echo "########## [$e] $q"
python3 search.py "$e" "$q" 2>&1 | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
except Exception as ex:
    print('parse err', ex); sys.exit()
if isinstance(d,dict): print('ERR', d); sys.exit()
for i,r in enumerate(d[:12]):
    print(i, '|', r['title'][:110], '|', r['url'][:130])
"
