// Built-in operations templates offered in the "New document" dialog.
// The HTML uses the node types the editor understands (headings, task
// lists, tables, code blocks). Addresses are examples only
// (example.com, 192.0.2.x).

export type OpsTemplateKey = 'release' | 'rollback' | 'dbchange' | 'certrenew' | 'onboarding'

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
// Inline text: `code` becomes <code>, everything else is escaped.
const inline = (s: string) => esc(s).replace(/`([^`]+)`/g, '<code>$1</code>')

const h2 = (s: string) => `<h2>${inline(s)}</h2>`
const h3 = (s: string) => `<h3>${inline(s)}</h3>`
const p = (s: string) => `<p>${inline(s)}</p>`
const ul = (items: string[]) => `<ul>${items.map(i => `<li><p>${inline(i)}</p></li>`).join('')}</ul>`
const ol = (items: string[]) => `<ol>${items.map(i => `<li><p>${inline(i)}</p></li>`).join('')}</ol>`
const tasks = (items: string[]) =>
  `<ul data-type="taskList">${items.map(i => `<li data-type="taskItem" data-checked="false"><p>${inline(i)}</p></li>`).join('')}</ul>`
const code = (lang: string, text: string) => `<pre><code class="language-${lang}">${esc(text)}</code></pre>`
const cell = (tag: 'th' | 'td', s: string) => `<${tag}><p>${inline(s)}</p></${tag}>`
// Table with a header row.
const table = (head: string[], rows: string[][]) =>
  `<table><tbody><tr>${head.map(h => cell('th', h)).join('')}</tr>${rows
    .map(r => `<tr>${r.map(c => cell('td', c)).join('')}</tr>`)
    .join('')}</tbody></table>`
// Two-column "item / value" table.
const kv = (head: [string, string], rows: [string, string][]) => table(head, rows)

const zh: Record<OpsTemplateKey, () => string> = {
  release: () => [
    h2('上线检查'),
    p('按「上线前 → 上线 → 上线后」的顺序走，每做完一项就勾上。前面没做完，不要往下走。'),
    h3('1. 基本信息'),
    kv(['项目', '内容'], [
      ['上线内容', '例：订单服务 v2.3.0，新增退款审核'],
      ['负责人', ''],
      ['上线时间', '例：2026-10-15 22:00～23:00（避开业务高峰）'],
      ['影响范围', '例：下单、退款接口，预计不停服'],
      ['出问题找谁', ''],
      ['回滚负责人', ''],
    ]),
    h3('2. 上线前'),
    tasks([
      '代码已经合并，版本号已经打好',
      '测试环境验证通过，测试同事已经确认',
      '有数据库改动的，已经按「数据库变更」模板单独过了一遍',
      '生产环境的配置已经准备好，密码、密钥没有写进代码和文档',
      '已经备份当前版本的程序、配置文件和要改的数据',
      '回滚步骤已经写好，确认上一个版本还能拿到',
      '已经通知相关同事和客服：什么时候上线、可能有什么影响',
      '监控和告警正常，上线时有人盯着',
    ]),
    h3('3. 上线步骤'),
    p('按实际情况改下面的命令，执行一步，记一步结果。'),
    code('bash', `# 1. 备份当前版本
sudo cp -a /opt/app/current /opt/app/backup-$(date +%Y%m%d%H%M)

# 2. 放上新版本（按你们的发布方式改）
sudo tar -xzf app-2.3.0.tar.gz -C /opt/app/releases/
sudo ln -sfn /opt/app/releases/app-2.3.0 /opt/app/current

# 3. 重启并看状态
sudo systemctl restart app
systemctl status app --no-pager

# 4. 健康检查
curl -fsS https://app.example.com/healthz`),
    h3('4. 上线后验证'),
    tasks([
      '健康检查通过',
      '核心流程手动走一遍（例：登录、下单、退款）',
      '错误日志里没有新出现的报错',
      '观察 15～30 分钟，接口报错的比例和响应时间正常',
      '告警群里没有新告警',
      '在群里通知上线完成',
    ]),
    h3('5. 结果记录'),
    table(['开始时间', '结束时间', '结果（成功 / 已回滚）', '遇到的问题'], [['', '', '', '']]),
    h3('6. 出问题怎么办'),
    p('先回滚，再查原因。核心功能用不了、报错明显变多，或者 15 分钟内找不到原因，就按「回滚」模板退回上一个版本。'),
  ].join(''),

  rollback: () => [
    h2('回滚'),
    p('上线后出了问题，先把服务恢复，再慢慢查原因。回滚前把这份文档填好，让大家知道在退什么、退到哪个版本。'),
    h3('1. 什么时候回滚'),
    ul([
      '核心功能用不了（例：登录不了、下不了单）',
      '报错或超时明显比上线前多',
      '数据写错了，而且还在继续写错',
      '15 分钟内找不到原因',
    ]),
    h3('2. 基本信息'),
    kv(['项目', '内容'], [
      ['当前版本', '例：v2.3.0'],
      ['退回到', '例：v2.2.4'],
      ['原因', ''],
      ['谁决定回滚', ''],
      ['谁来执行', ''],
      ['开始时间', ''],
    ]),
    h3('3. 回滚前确认'),
    tasks([
      '上一个版本的程序或镜像还在，可以直接用',
      '这次上线改没改数据库？改了的话，先确认旧版本能不能用改过的表；不能的话不要直接回滚程序，先找负责人商量',
      '配置文件改没改？改了的话一起退回',
      '已经在群里说正在回滚，避免别人同时操作',
    ]),
    h3('4. 回滚步骤'),
    p('直接部署在服务器上的：'),
    code('bash', `# 找到上线前的备份
ls -lt /opt/app/

# 切回旧版本并重启
sudo ln -sfn /opt/app/backup-202610152200 /opt/app/current
sudo systemctl restart app
systemctl status app --no-pager`),
    p('用 Docker 部署的：'),
    code('bash', `# 把 docker-compose.yml 里的镜像版本改回上一个，比如
#   image: registry.example.com/app:2.2.4
docker compose up -d
docker compose ps`),
    p('配置文件也要退回的：'),
    code('bash', `sudo cp /etc/app/config.yaml.bak /etc/app/config.yaml
sudo systemctl restart app`),
    h3('5. 回滚后验证'),
    tasks([
      '健康检查通过：`curl -fsS https://app.example.com/healthz`',
      '核心流程手动走一遍',
      '报错数量回到上线前的水平',
      '确认现在跑的确实是旧版本',
    ]),
    h3('6. 事后'),
    tasks([
      '在群里通知回滚完成，说明影响了多长时间',
      '记下这次出问题的原因：直接原因是什么，为什么测试时没发现',
      '回滚前写错的数据，单独列出来处理',
      '改好以后，重新按「上线检查」走一遍',
    ]),
  ].join(''),

  dbchange: () => [
    h2('数据库变更'),
    p('改表结构、批量改数据、删数据，都按这份走。三条原则：先备份，先查行数，能退回。'),
    h3('1. 基本信息'),
    kv(['项目', '内容'], [
      ['数据库', '例：db.example.com / app_db'],
      ['涉及的表', '例：orders'],
      ['变更类型', '加字段 / 加索引 / 改数据 / 删数据 / 删表'],
      ['预计影响行数', ''],
      ['执行人', ''],
      ['复核人', ''],
      ['执行时间', '例：2026-10-15 23:00（业务低峰）'],
    ]),
    h3('2. 要执行的语句'),
    code('sql', `-- 例：给订单表加一个退款原因字段
ALTER TABLE orders ADD COLUMN refund_reason VARCHAR(255) NULL DEFAULT NULL;`),
    h3('3. 执行前检查'),
    tasks([
      '语句已经在测试库跑过，记下了用时',
      '表很大（例：超过 100 万行）时，已经估过加字段、加索引会锁表多久，必要时放到低峰期，或者换不锁表的做法',
      '改数据、删数据的语句都带了 WHERE 条件，不会误改整张表',
      '已经先用 SELECT 查过要改多少行，和预计的一致',
      '程序的新旧两个版本都能用改完的表（先改库还是先上线，已经想清楚）',
      '已经备份要改的表（见第 4 步）',
      '退回用的语句已经写好（见第 6 步）',
    ]),
    h3('4. 备份'),
    code('bash', `mysqldump --single-transaction -h db.example.com -u admin -p app_db orders > orders-$(date +%Y%m%d%H%M).sql
ls -lh orders-*.sql`),
    h3('5. 执行'),
    code('sql', `-- 1. 先查要改的行数
SELECT COUNT(*) FROM orders WHERE status = 'expired' AND created_at < '2026-01-01';

-- 2. 在事务里改
BEGIN;
UPDATE orders SET status = 'archived' WHERE status = 'expired' AND created_at < '2026-01-01';
-- 3. 看影响行数，和第 1 步一样再提交；不一样就执行 ROLLBACK;
COMMIT;`),
    p('注意：MySQL 里的 ALTER TABLE、DROP TABLE 这类改表结构的语句，执行了就撤不回来，只能靠备份和退回语句。'),
    h3('6. 退回方案'),
    code('sql', `-- 例：去掉刚加的字段
ALTER TABLE orders DROP COLUMN refund_reason;

-- 例：用备份恢复整张表（会覆盖这张表现在的数据，先确认）
-- mysql -h db.example.com -u admin -p app_db < orders-202610152300.sql`),
    h3('7. 执行后检查'),
    tasks([
      '抽查几条数据，确认改对了',
      '程序日志里没有新的数据库报错',
      '慢查询没有变多',
      '备份文件放到约定的位置，至少留 7 天',
    ]),
    h3('8. 执行记录'),
    table(['开始时间', '结束时间', '实际影响行数', '用时', '结果', '备注'], [['', '', '', '', '', '']]),
  ].join(''),

  certrenew: () => [
    h2('HTTPS 证书续期'),
    p('证书过期后，浏览器会直接拦住用户，App 和接口调用也会失败。所有证书都登记在这里，到期前 30 天开始处理。'),
    h3('1. 证书清单'),
    table(['域名', '到期日', '放在哪台机器、哪个路径', '用在哪里', '怎么续', '负责人'], [
      ['www.example.com', '例：2026-12-31', '192.0.2.10:/etc/nginx/ssl/', 'nginx', '自动续期（certbot）', ''],
      ['api.example.com', '', '192.0.2.11:/etc/nginx/ssl/', 'nginx、负载均衡', '手动购买', ''],
    ]),
    h3('2. 查看到期时间'),
    code('bash', `# 查网站现在正在用的证书
echo | openssl s_client -connect www.example.com:443 -servername www.example.com 2>/dev/null | openssl x509 -noout -subject -enddate

# 查服务器上的证书文件
openssl x509 -in /etc/nginx/ssl/www.example.com.crt -noout -enddate`),
    h3('3. 自动续期的（certbot）'),
    code('bash', `# 先试一次，不会真的换
sudo certbot renew --dry-run

# 正式续期，然后让 nginx 重新加载
sudo certbot renew
sudo nginx -t && sudo systemctl reload nginx`),
    h3('4. 手动购买的'),
    ol([
      '在买证书的平台续费或重新申请，下载 nginx 格式的证书文件',
      '备份旧证书：`sudo cp -a /etc/nginx/ssl /etc/nginx/ssl.bak-$(date +%Y%m%d)`',
      '把新证书和私钥放到原来的位置，文件名不变',
      '检查配置并重新加载：`sudo nginx -t && sudo systemctl reload nginx`',
      '用第 2 步的命令确认到期时间已经是新的',
    ]),
    h3('5. 换完检查'),
    tasks([
      '证书文件里带上了中间证书（证书链完整），不然有些手机和老系统会打不开',
      '私钥文件只有 root 能读（权限 600）',
      '同一张证书用在好几个地方的（CDN、负载均衡、其他服务器），每个地方都换了',
      '用浏览器打开网站，看到的到期时间是新的',
      '更新上面证书清单里的到期日',
      '在交期看板上建一条下次到期的提醒，提前 30 天',
    ]),
    h3('6. 续期记录'),
    table(['日期', '域名', '新的到期日', '操作人', '备注'], [['', '', '', '', '']]),
  ].join(''),

  onboarding: () => [
    h2('新人入职'),
    p('带教人照着这份清单，帮新同事开通账号、熟悉系统。权限按需给，用不到的先不开。'),
    h3('1. 基本信息'),
    kv(['项目', '内容'], [
      ['姓名', ''],
      ['岗位', ''],
      ['入职日期', ''],
      ['带教人', ''],
      ['试用期到', ''],
    ]),
    h3('2. 第一天：开通账号'),
    tasks([
      '公司邮箱和聊天工具，拉进团队群和告警群',
      '加入 MistLab 团队，先给「查看者」或「编辑者」，不给管理员',
      '装好 MistTerm，只添加工作需要的服务器',
      '登录服务器用个人账号和 SSH 密钥，不共用账号，不发密码',
      '开通代码仓库、监控、日志平台的账号',
      '有跳板机或 VPN 的，开通个人账号',
    ]),
    h3('3. 第一周：熟悉系统'),
    tasks([
      '看一遍服务清单：有哪些服务、各自在哪台机器、谁负责',
      '分清测试环境和生产环境，知道生产上的操作要格外小心',
      '读完团队的几份手册：上线检查、回滚、数据库变更、证书续期',
      '跟着带教人做一次上线，按「上线检查」走',
      '知道告警怎么收、值班怎么排、半夜出问题找谁',
    ]),
    h3('4. 常用地址'),
    table(['名称', '地址', '说明'], [
      ['测试环境', 'https://test.example.com', '可以放心试'],
      ['生产环境', 'https://www.example.com', '改动前先在测试环境试一遍'],
      ['监控', 'https://monitor.example.com', ''],
      ['跳板机', '192.0.2.20', '用个人账号登录'],
      ['团队文档', '就是这里', '手册和操作记录都放这里'],
    ]),
    h3('5. 几条规矩'),
    ul([
      '生产环境的操作，先在测试环境做一遍',
      '删除、重启、改配置之前，先跟带教人说一声',
      '密码和密钥不要发在聊天里，也不要写进文档',
      '拿不准的命令不要直接执行，先问',
    ]),
    h3('6. 开通记录'),
    p('离职或转岗时，按这张表把权限一项项收回。'),
    table(['系统', '开通的权限', '开通日期', '批准人', '收回日期'], [['', '', '', '', '']]),
  ].join(''),
}

const en: Record<OpsTemplateKey, () => string> = {
  release: () => [
    h2('Release checklist'),
    p('Work through it in order: before, during, after. Tick each item when it is done. Do not move on while something above is still open.'),
    h3('1. Overview'),
    kv(['Item', 'Details'], [
      ['What is going out', 'e.g. Order service v2.3.0, adds refund review'],
      ['Owner', ''],
      ['Window', 'e.g. 2026-10-15 22:00–23:00 (outside peak hours)'],
      ['Impact', 'e.g. order and refund APIs, no downtime expected'],
      ['Who to call if it breaks', ''],
      ['Rollback owner', ''],
    ]),
    h3('2. Before'),
    tasks([
      'Code is merged and the version is tagged',
      'Verified on the test environment and signed off by QA',
      'Any database change has gone through the "Database change" template',
      'Production config is ready; no passwords or keys in code or docs',
      'Backed up the current build, config files and any data that will change',
      'Rollback steps are written and the previous build is still available',
      'Colleagues and support know when it goes out and what may be affected',
      'Monitoring and alerts are working, and someone is watching',
    ]),
    h3('3. Steps'),
    p('Adapt the commands, run one step at a time and note the result.'),
    code('bash', `# 1. Back up the current build
sudo cp -a /opt/app/current /opt/app/backup-$(date +%Y%m%d%H%M)

# 2. Put the new build in place (adapt to how you ship)
sudo tar -xzf app-2.3.0.tar.gz -C /opt/app/releases/
sudo ln -sfn /opt/app/releases/app-2.3.0 /opt/app/current

# 3. Restart and check
sudo systemctl restart app
systemctl status app --no-pager

# 4. Health check
curl -fsS https://app.example.com/healthz`),
    h3('4. After'),
    tasks([
      'Health check passes',
      'Walk through the key flows by hand (e.g. sign in, order, refund)',
      'No new errors in the logs',
      'Watch for 15–30 minutes: error rate and response times are normal',
      'No new alerts',
      'Announce that the release is done',
    ]),
    h3('5. Record'),
    table(['Start', 'End', 'Result (done / rolled back)', 'Problems'], [['', '', '', '']]),
    h3('6. If something goes wrong'),
    p('Roll back first, investigate later. If a key feature is broken, errors clearly go up, or you cannot find the cause within 15 minutes, follow the "Rollback" template.'),
  ].join(''),

  rollback: () => [
    h2('Rollback'),
    p('When a release goes wrong, restore service first and find the cause later. Fill this in before you start so everyone knows what is being rolled back and to which version.'),
    h3('1. When to roll back'),
    ul([
      'A key feature is broken (e.g. nobody can sign in or place orders)',
      'Errors or timeouts are clearly higher than before the release',
      'Data is being written wrong and it keeps happening',
      'No cause found within 15 minutes',
    ]),
    h3('2. Overview'),
    kv(['Item', 'Details'], [
      ['Current version', 'e.g. v2.3.0'],
      ['Roll back to', 'e.g. v2.2.4'],
      ['Reason', ''],
      ['Decided by', ''],
      ['Carried out by', ''],
      ['Started at', ''],
    ]),
    h3('3. Before rolling back'),
    tasks([
      'The previous build or image is still available',
      'Did the release change the database? If so, check the old version works with the changed tables. If it does not, do not just roll back the code; talk to the owner first',
      'Did config files change? If so, roll them back too',
      'Announced the rollback so nobody else changes things at the same time',
    ]),
    h3('4. Steps'),
    p('Deployed directly on the server:'),
    code('bash', `# Find the backup taken before the release
ls -lt /opt/app/

# Switch back and restart
sudo ln -sfn /opt/app/backup-202610152200 /opt/app/current
sudo systemctl restart app
systemctl status app --no-pager`),
    p('Deployed with Docker:'),
    code('bash', `# Set the image in docker-compose.yml back to the previous version, e.g.
#   image: registry.example.com/app:2.2.4
docker compose up -d
docker compose ps`),
    p('Config files as well:'),
    code('bash', `sudo cp /etc/app/config.yaml.bak /etc/app/config.yaml
sudo systemctl restart app`),
    h3('5. Check after rolling back'),
    tasks([
      'Health check passes: `curl -fsS https://app.example.com/healthz`',
      'Walk through the key flows by hand',
      'Errors are back to the level before the release',
      'Confirm the old version is what is running now',
    ]),
    h3('6. Afterwards'),
    tasks([
      'Announce that the rollback is done and how long users were affected',
      'Write down the cause and why testing did not catch it',
      'List and fix any data written wrong before the rollback',
      'Once fixed, go through the "Release checklist" again',
    ]),
  ].join(''),

  dbchange: () => [
    h2('Database change'),
    p('Use this for schema changes, bulk updates and deletes. Three rules: back up first, count rows first, have a way back.'),
    h3('1. Overview'),
    kv(['Item', 'Details'], [
      ['Database', 'e.g. db.example.com / app_db'],
      ['Tables', 'e.g. orders'],
      ['Type of change', 'add column / add index / update data / delete data / drop table'],
      ['Expected rows affected', ''],
      ['Run by', ''],
      ['Reviewed by', ''],
      ['When', 'e.g. 2026-10-15 23:00 (low traffic)'],
    ]),
    h3('2. Statements'),
    code('sql', `-- e.g. add a refund reason column to orders
ALTER TABLE orders ADD COLUMN refund_reason VARCHAR(255) NULL DEFAULT NULL;`),
    h3('3. Before running'),
    tasks([
      'Ran on the test database and noted how long it took',
      'For large tables (e.g. over 1 million rows), estimated how long adding a column or index locks the table; scheduled for low traffic or used a non-locking method',
      'Every UPDATE and DELETE has a WHERE clause',
      'Counted the affected rows with SELECT and the number matches',
      'Both the old and new app versions work with the changed tables',
      'Backed up the tables (step 4)',
      'Wrote the statements to undo it (step 6)',
    ]),
    h3('4. Backup'),
    code('bash', `mysqldump --single-transaction -h db.example.com -u admin -p app_db orders > orders-$(date +%Y%m%d%H%M).sql
ls -lh orders-*.sql`),
    h3('5. Run'),
    code('sql', `-- 1. Count the rows first
SELECT COUNT(*) FROM orders WHERE status = 'expired' AND created_at < '2026-01-01';

-- 2. Change them inside a transaction
BEGIN;
UPDATE orders SET status = 'archived' WHERE status = 'expired' AND created_at < '2026-01-01';
-- 3. Commit only if the affected rows match step 1; otherwise run ROLLBACK;
COMMIT;`),
    p('Note: in MySQL, schema statements such as ALTER TABLE and DROP TABLE cannot be undone by a transaction. Only the backup and the undo statements can bring things back.'),
    h3('6. How to undo'),
    code('sql', `-- e.g. remove the new column
ALTER TABLE orders DROP COLUMN refund_reason;

-- e.g. restore the whole table from the backup (overwrites its current data, confirm first)
-- mysql -h db.example.com -u admin -p app_db < orders-202610152300.sql`),
    h3('7. After running'),
    tasks([
      'Spot-check a few rows',
      'No new database errors in the app logs',
      'No increase in slow queries',
      'Backup stored in the agreed place and kept for at least 7 days',
    ]),
    h3('8. Record'),
    table(['Start', 'End', 'Rows affected', 'Duration', 'Result', 'Notes'], [['', '', '', '', '', '']]),
  ].join(''),

  certrenew: () => [
    h2('HTTPS certificate renewal'),
    p('When a certificate expires, browsers block users and apps and API calls fail. List every certificate here and start renewing 30 days before it expires.'),
    h3('1. Certificates'),
    table(['Domain', 'Expires', 'Server and path', 'Used by', 'Renewal', 'Owner'], [
      ['www.example.com', 'e.g. 2026-12-31', '192.0.2.10:/etc/nginx/ssl/', 'nginx', 'automatic (certbot)', ''],
      ['api.example.com', '', '192.0.2.11:/etc/nginx/ssl/', 'nginx, load balancer', 'bought manually', ''],
    ]),
    h3('2. Check the expiry date'),
    code('bash', `# The certificate the site is serving right now
echo | openssl s_client -connect www.example.com:443 -servername www.example.com 2>/dev/null | openssl x509 -noout -subject -enddate

# A certificate file on the server
openssl x509 -in /etc/nginx/ssl/www.example.com.crt -noout -enddate`),
    h3('3. Automatic renewal (certbot)'),
    code('bash', `# Dry run first, nothing is replaced
sudo certbot renew --dry-run

# Renew, then reload nginx
sudo certbot renew
sudo nginx -t && sudo systemctl reload nginx`),
    h3('4. Bought manually'),
    ol([
      'Renew or re-issue it where you bought it and download the nginx version',
      'Back up the old one: `sudo cp -a /etc/nginx/ssl /etc/nginx/ssl.bak-$(date +%Y%m%d)`',
      'Put the new certificate and key in the same place with the same file names',
      'Check and reload: `sudo nginx -t && sudo systemctl reload nginx`',
      'Run the commands in step 2 and confirm the new expiry date',
    ]),
    h3('5. Check after replacing'),
    tasks([
      'The file includes the intermediate certificate (full chain), or some phones and older systems will refuse it',
      'The private key is readable by root only (mode 600)',
      'Every place that uses the same certificate (CDN, load balancer, other servers) has the new one',
      'The browser shows the new expiry date',
      'Updated the expiry date in the list above',
      'Added a reminder on the deadline board, 30 days before the next expiry',
    ]),
    h3('6. Renewal log'),
    table(['Date', 'Domain', 'New expiry', 'Done by', 'Notes'], [['', '', '', '', '']]),
  ].join(''),

  onboarding: () => [
    h2('New team member onboarding'),
    p('The mentor uses this list to set up accounts and walk the new colleague through the systems. Grant access only as it is needed.'),
    h3('1. Overview'),
    kv(['Item', 'Details'], [
      ['Name', ''],
      ['Role', ''],
      ['Start date', ''],
      ['Mentor', ''],
      ['Probation ends', ''],
    ]),
    h3('2. Day one: accounts'),
    tasks([
      'Company email and chat; add to the team channel and the alerts channel',
      'Join the MistLab team as Viewer or Editor, not Admin',
      'Install MistTerm and add only the servers they need',
      'Personal server accounts with SSH keys; no shared accounts, no passwords in chat',
      'Accounts for the code repository, monitoring and logs',
      'Personal account for the jump host or VPN, if you have one',
    ]),
    h3('3. First week: learn the systems'),
    tasks([
      'Go through the service list: what runs where and who owns it',
      'Know how to tell test from production, and that production needs extra care',
      'Read the team runbooks: release checklist, rollback, database change, certificate renewal',
      'Do one release together with the mentor, following the release checklist',
      'Know how alerts arrive, how on-call works and who to call at night',
    ]),
    h3('4. Useful addresses'),
    table(['Name', 'Address', 'Notes'], [
      ['Test environment', 'https://test.example.com', 'Safe to experiment'],
      ['Production', 'https://www.example.com', 'Try changes on test first'],
      ['Monitoring', 'https://monitor.example.com', ''],
      ['Jump host', '192.0.2.20', 'Sign in with your own account'],
      ['Team docs', 'Right here', 'Runbooks and records live here'],
    ]),
    h3('5. House rules'),
    ul([
      'Do anything on production on the test environment first',
      'Tell your mentor before deleting, restarting or changing config',
      'Never post passwords or keys in chat or in documents',
      'If you are not sure about a command, ask before running it',
    ]),
    h3('6. Access log'),
    p('When someone leaves or changes role, use this table to remove each access.'),
    table(['System', 'Access granted', 'Granted on', 'Approved by', 'Removed on'], [['', '', '', '', '']]),
  ].join(''),
}

export const opsTemplateKeys: OpsTemplateKey[] = ['release', 'rollback', 'dbchange', 'certrenew', 'onboarding']

export function isOpsTemplate(key: string): key is OpsTemplateKey {
  return (opsTemplateKeys as string[]).includes(key)
}

export function opsTemplateHTML(key: OpsTemplateKey, locale: string): string {
  return (locale.toLowerCase().startsWith('zh') ? zh : en)[key]()
}
