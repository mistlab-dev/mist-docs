<template>
  <div class="help-page">
    <div class="help-header">
      <h1><el-icon :size="28" style="vertical-align: middle;margin-right:8px"><QuestionFilled /></el-icon>{{ t('help.title') }}</h1>
      <p>{{ t('help.subtitle') }}</p>
    </div>

    <div class="help-search">
      <el-input v-model="keyword" :placeholder="t('help.searchPlaceholder')" prefix-icon="Search" size="large" clearable />
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
        <el-empty :description="t('help.noResults')" />
      </div>
    </div>

    <div class="help-footer">
      <p>{{ t('help.footer') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, QuestionFilled } from '@element-plus/icons-vue'

const { t, tm } = useI18n()

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

const sections = computed<HelpSection[]>(() => {
  const helpSections = tm('help.sections') as Record<string, any>
  if (!helpSections) return []

  const icons: Record<string, string> = {
    docManagement: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 3h6v2H7V7zm0 4h4v2H7v-2z"/></svg>',
    shareCollab: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M12.586 4.586a2 2 0 112.828 2.828l-3.879 3.879a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3.879-3.879a4 4 0 00-5.656-5.656L8.12 5.464a1 1 0 001.414 1.414l3.052-3.292z"/><path d="M7.414 15.414a2 2 0 11-2.828-2.828l3.879-3.879a2 2 0 012.828 0 1 1 0 001.414-1.414 4 4 0 00-5.656 0L3.172 11.17a4 4 0 005.656 5.656l2.828-2.828a1 1 0 10-1.414-1.414l-2.828 2.83z"/></svg>',
    sheetEditor: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><rect x="3" y="3" width="14" height="14" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><line x1="3" y1="7" x2="17" y2="7" stroke="currentColor" stroke-width="1"/><line x1="3" y1="11" x2="17" y2="11" stroke="currentColor" stroke-width="1"/><line x1="8" y1="7" x2="8" y2="17" stroke="currentColor" stroke-width="1"/></svg>',
    security: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2z"/></svg>',
    adminSection: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M10 2a8 8 0 100 16 8 8 0 000-16zm1 11H9v-2h2v2zm0-4H9V5h2v4z"/></svg>',
    mobile: '<svg viewBox="0 0 20 20" fill=\'currentColor\' width=\'20\' height=\'20\'><path d="M3 5a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V5zm2 0v8h10V5H5zm1 12h8v2H6v-2z"/></svg>',
  }

  const result: HelpSection[] = []

  for (const [sectionKey, sectionData] of Object.entries(helpSections)) {
    const section: HelpSection = {
      icon: icons[sectionKey] || '',
      title: t(`help.sections.${sectionKey}.title`),
      items: [],
    }

    if (sectionData && typeof sectionData === 'object' && sectionData.items) {
      const items = sectionData.items as Record<string, any>
      for (const [itemKey, itemData] of Object.entries(items)) {
        if (itemData && typeof itemData === 'object') {
          section.items.push({
            q: t(`help.sections.${sectionKey}.items.${itemKey}.q`),
            a: t(`help.sections.${sectionKey}.items.${itemKey}.a`),
          })
        }
      }
    }

    result.push(section)
  }

  return result
})

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
