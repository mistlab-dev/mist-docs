<template>
  <el-dropdown trigger="click" @command="switchLocale">
    <span class="lang-switch">
      <el-icon><span style="font-size:14px">🌐</span></el-icon>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="zh-CN" :class="{ active: currentLocale === 'zh-CN' }">中文</el-dropdown-item>
        <el-dropdown-item command="en-US" :class="{ active: currentLocale === 'en-US' }">English</el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { setLocale, getLocale } from '@/i18n'

const { locale } = useI18n()
const currentLocale = computed(() => locale.value)

function switchLocale(lang: string) {
  setLocale(lang)
}
</script>

<style scoped>
.lang-switch {
  cursor: pointer;
  display: flex;
  align-items: center;
  padding: 4px;
}
.lang-switch:hover {
  opacity: 0.8;
}
</style>
