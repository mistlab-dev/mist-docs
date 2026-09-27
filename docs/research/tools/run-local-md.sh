#!/bin/bash
# 本地端到端验证：启动 mist-docs 并触发一次提醒扫描
# 用途：验证交期看板链路（建表 / 接口 / 看板视图 / 提醒幂等）
cd /root/work/mist-docs || exit 1

pkill -f mist-docs-local 2>/dev/null
sleep 2

export MISTDOCS_MASTER_KEY="$(cat /root/work/secrets/mistdocs-master.key)"
nohup setsid /tmp/mist-docs-local -c configs/config.yaml > /tmp/md-local2.log 2>&1 < /dev/null &
echo "started pid=$!"
