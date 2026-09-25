<template>
  <el-config-provider :locale="epLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElConfigProvider } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import { useI18n } from 'vue-i18n'
import { useTheme } from '@/composables/useTheme'

useTheme()

const { locale } = useI18n()

const epLocale = computed(() => {
  return locale.value === 'en-US' ? en : zhCn
})
</script>

<style>
/* iOS Safari 100vh fix */
:root {
  --vh: 1vh;
}
html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
}
body {
  font-family: 'Noto Sans SC', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: var(--md-bg, #f5f7fa);
}

/* Dark theme overrides are centralized in src/styles/dark.css */
</style>
