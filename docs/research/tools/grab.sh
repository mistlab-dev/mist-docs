#!/bin/bash
# usage: grab.sh <outfile> <baidu-link-or-url>
out="$1"; u="$2"
{ echo "===== SRC: $u"; /root/work/research/tools/bd_article.sh "$u" 8000; echo; } >> "$out" 2>&1
