#!/bin/bash
# usage: run.sh <outfile> <query> [page]
out="$1"; shift
q="$1"; p="${2:-1}"
{ echo "########## Q: $q (p$p)"; timeout 200 python3 /root/work/research/tools/wx.py "$q" "$p" 2>&1; } >> "$out"
