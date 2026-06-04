<template>
  <div class="help-page">
    <div class="help-header">
      <h1><el-icon :size="28" style="vertical-align: middle;margin-right:8px"><QuestionFilled /></el-icon>帮助中心</h1>
      <p>MistDocs 使用指南</p>
    </div>

    <div class="help-search">
      <el-input v-model="keyword" placeholder="搜索帮助内容..." prefix-icon="Search" size="large" clearable />
    </div>

    <div class="help-content">
      <template v-for="section in filteredSections" :key="section.title">
        <div class="help-section">
          <h2><span class="section-icon" v-html="section.icon"></span> {{ section.title }}</h2>
          <div class="help-cards">
            <div v-for="item in section.items" :key="item.q" class="help-card" @click="toggleItem(item)">
              <div class="help-card-header">
                <span class="help-q">{{ item.q }}</span>
                <el-icon :class="{ rotated: item.open }"><ArrowDown /></el-icon>
              </div>
              <div v-if="item.open" class="help-card-body" v-html="item.a"></div>
            </div>
          </div>
        </div>
      </template>

      <div v-if="!filteredSections.length" class="help-empty">
        <el-empty description="没有找到相关内容" />
      </div>
    </div>

    <div class="help-footer">
      <p>仍有疑问？请联系管理员</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ArrowDown, QuestionFilled } from '@element-plus/icons-vue'

interface HelpItem {
  q: string
  a: string
  open?: boolean
}

interface HelpSection {
  icon: string
  title: string
  items: HelpItem[]
}

const keyword = ref('')

const sections = ref<HelpSection[]>([
  {
    icon: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 3h6v2H7V7zm0 4h4v2H7v-2z"/></svg>',
    title: '文档管理',
    items: [
      { q: '如何创建文档？', a: '<p>点击左侧「文档」进入文档列表，然后点击右上角「新建」按钮。可选择创建：<br><b>文档</b> — 富文本编辑器，适合写文章、报告<br><b>表格</b> — 类似 Excel 的电子表格<br><b>Markdown</b> — 支持 Markdown 语法的编辑器</p>' },
      { q: '如何编辑文档？', a: '<p>在文档列表中点击文档名称即可进入编辑。编辑器会自动保存，也可以点击「保存」按钮手动保存。</p>' },
      { q: '如何删除文档？', a: '<p>在文档列表中，将鼠标悬停在文档上，点击右侧「⋮」菜单中的「删除」。删除后的文档会进入回收站，30天内可恢复。</p>' },
      { q: '如何移动文档到文件夹？', a: '<p>在文档列表中，点击文档右侧「⋮」菜单 →「移动」，选择目标文件夹即可。</p>' },
      { q: '如何恢复已删除的文档？', a: '<p>点击左侧「回收站」，找到要恢复的文档，点击「恢复」按钮。文档会恢复到原来的位置。</p>' },
      { q: '支持哪些文档类型？', a: '<p>目前支持三种类型：<br><b>文档</b> — 富文本（标题、列表、代码块、图片等）<br><b>表格</b> — 电子表格（公式、图表、数据透视等）<br><b>Markdown</b> — 标准 Markdown 语法</p>' },
    ]
  },
  {
    icon: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M12.586 4.586a2 2 0 112.828 2.828l-3.879 3.879a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3.879-3.879a4 4 0 00-5.656-5.656L8.12 5.464a1 1 0 001.414 1.414l3.052-3.292z"/><path d="M7.414 15.414a2 2 0 11-2.828-2.828l3.879-3.879a2 2 0 012.828 0 1 1 0 001.414-1.414 4 4 0 00-5.656 0L3.172 11.17a4 4 0 005.656 5.656l2.828-2.828a1 1 0 10-1.414-1.414l-2.828 2.83z"/></svg>',
    title: '分享与协作',
    items: [
      { q: '如何分享文档给他人？', a: '<p>打开文档后，点击右上角「分享」按钮。可以设置协作者的权限：<br><b>查看者</b> — 只能查看文档内容<br><b>编辑者</b> — 可以编辑文档内容<br><b>管理员</b> — 可以编辑、分享、管理权限</p>' },
      { q: '如何生成分享链接？', a: '<p>在分享弹窗中，开启「链接分享」即可生成分享链接。可以设置链接的访问权限（查看/编辑）。</p>' },
      { q: '多人可以同时编辑吗？', a: '<p>是的！MistDocs 支持多人实时协作编辑。打开同一文档的用户可以看到彼此的编辑内容实时同步。</p>' },
      { q: '如何查看文档历史版本？', a: '<p>打开文档后，点击右上角「⋮」菜单 →「版本历史」，可以查看所有历史版本并恢复到任意版本。</p>' },
    ]
  },
  {
    icon: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><rect x="3" y="3" width="14" height="14" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><line x1="3" y1="7" x2="17" y2="7" stroke="currentColor" stroke-width="1"/><line x1="3" y1="11" x2="17" y2="11" stroke="currentColor" stroke-width="1"/><line x1="8" y1="7" x2="8" y2="17" stroke="currentColor" stroke-width="1"/></svg>',
    title: '表格编辑器',
    items: [
      { q: '如何在单元格中输入内容？', a: '<p><b>电脑</b>：双击单元格或直接输入文字<br><b>手机</b>：点击选中单元格，再点击一次即可输入</p>' },
      { q: '如何使用公式？', a: '<p>选中单元格，在顶部公式栏输入以 <code>=</code> 开头的公式。例如：<br><code>=SUM(A1:A10)</code> 求和<br><code>=AVERAGE(B1:B5)</code> 平均值<br><code>=IF(A1>0,"正","负")</code> 条件判断</p>' },
      { q: '如何插入图表？', a: '<p>选中要生成图表的数据区域，点击工具栏「插入图表」按钮，选择图表类型（柱状图、折线图、饼图等）。</p>' },
      { q: '如何合并单元格？', a: '<p>选中要合并的单元格区域，点击工具栏「合并单元格」按钮。</p>' },
      { q: '如何冻结行/列？', a: '<p>点击工具栏「冻结窗格」按钮，选择冻结首行、首列或当前区域。</p>' },
      { q: '如何导出数据？', a: '<p>点击工具栏「导出CSV」按钮，可以将当前表格数据导出为 CSV 文件。</p>' },
    ]
  },
  {
    icon: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2z"/></svg>',
    title: '安全与隐私',
    items: [
      { q: '我的数据安全吗？', a: '<p>是的，所有文档都经过加密存储。MistDocs 使用端到端加密技术保护您的数据安全。只有有权限的用户才能访问文档内容。</p>' },
      { q: '水印是什么？', a: '<p>水印是覆盖在文档上的半透明文字，显示当前用户名和时间。普通用户查看文档时会自动显示水印，用于防止截图泄露。管理员可以手动开启或关闭水印。</p>' },
      { q: '如何修改密码？', a: '<p>请联系系统管理员修改密码。</p>' },
    ]
  },
  {
    icon: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M10 2a8 8 0 100 16 8 8 0 000-16zm1 11H9v-2h2v2zm0-4H9V5h2v4z"/></svg>',
    title: '系统管理（管理员）',
    items: [
      { q: '如何管理用户？', a: '<p>管理员可通过「用户管理」页面添加、编辑、禁用用户账号，以及重置密码。</p>' },
      { q: '如何管理部门？', a: '<p>管理员可通过「部门管理」页面创建和管理部门。每个部门可以有部门管理员，负责管理本部门的文档和成员。</p>' },
      { q: '如何查看审计日志？', a: '<p>管理员可通过「审计日志」页面查看所有用户操作记录，包括登录、文档创建、编辑、删除、分享等操作。</p>' },
      { q: '如何监控存储？', a: '<p>管理员可通过「存储监控」页面查看系统存储使用情况、各用户和部门的存储占用。</p>' },
      { q: '如何备份和恢复？', a: '<p>系统会自动定期备份。如需手动备份或恢复，请联系运维人员。</p>' },
    ]
  },
  {
    icon: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M3 5a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V5zm2 0v8h10V5H5zm1 12h8v2H6v-2z"/></svg>',
    title: '移动端使用',
    items: [
      { q: '手机上可以使用吗？', a: '<p>可以！MistDocs 支持手机浏览器访问。直接在手机浏览器中打开系统地址即可使用，无需安装 App。</p>' },
      { q: '手机上如何编辑表格？', a: '<p>1. 点击单元格选中（显示蓝色边框）<br>2. 再次点击同一单元格，即可输入内容<br>3. 虚拟键盘弹出后开始输入<br>4. 点击「✓」或按回车确认输入</p>' },
      { q: '手机上编辑文档卡顿怎么办？', a: '<p>建议使用 Chrome 或 Safari 最新版本。如果仍然卡顿，可以尝试刷新页面。大型文档建议在电脑上编辑。</p>' },
    ]
  },
])

function toggleItem(item: HelpItem) {
  item.open = !item.open
}

const filteredSections = computed(() => {
  if (!keyword.value.trim()) return sections.value
  const kw = keyword.value.toLowerCase()
  return sections.value
    .map(s => ({
      ...s,
      items: s.items.filter(i => i.q.toLowerCase().includes(kw) || i.a.toLowerCase().includes(kw))
    }))
    .filter(s => s.items.length > 0)
})
</script>

<style scoped>
.help-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 32px 24px;
}

.help-header {
  text-align: center;
  margin-bottom: 32px;
}
.help-header h1 {
  font-size: 28px;
  font-weight: 700;
  color: #303133;
  margin: 0 0 8px;
}
.help-header p {
  color: #909399;
  font-size: 15px;
  margin: 0;
}

.help-search {
  margin-bottom: 32px;
}

.help-section {
  margin-bottom: 28px;
}
.help-section h2 {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #ebeef5;
  display: flex;
  align-items: center;
  gap: 8px;
}
.section-icon {
  display: inline-flex;
  color: #409eff;
}

.help-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.help-card {
  background: #fff;
  border: 1px solid #e8ecf0;
  border-radius: 8px;
  padding: 14px 18px;
  cursor: pointer;
  transition: all 0.2s;
}
.help-card:hover {
  border-color: #409eff;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.1);
}

.help-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.help-q {
  font-size: 15px;
  font-weight: 500;
  color: #303133;
}
.help-card-header .el-icon {
  transition: transform 0.2s;
  color: #c0c4cc;
}
.help-card-header .el-icon.rotated {
  transform: rotate(180deg);
}

.help-card-body {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f0f2f5;
  font-size: 14px;
  color: #606266;
  line-height: 1.8;
}
.help-card-body :deep(code) {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 13px;
  color: #e6a23c;
}
.help-card-body :deep(b) {
  color: #303133;
}

.help-empty {
  padding: 60px 0;
}

.help-footer {
  text-align: center;
  margin-top: 40px;
  padding: 20px 0;
  border-top: 1px solid #ebeef5;
  color: #909399;
  font-size: 14px;
}

@media (max-width: 768px) {
  .help-page {
    padding: 20px 16px;
  }
  .help-header h1 {
    font-size: 22px;
  }
  .help-card {
    padding: 12px 14px;
  }
}
</style>
