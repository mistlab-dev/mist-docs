<template>
  <!-- A button the current role may not use: disabled, with the reason on hover.
       The span keeps the tooltip working (disabled buttons get no mouse events). -->
  <el-tooltip :disabled="allowed" :content="reason" placement="top">
    <span class="guarded-btn">
      <el-button v-bind="$attrs" :disabled="!allowed || disabled"><slot /></el-button>
    </span>
  </el-tooltip>
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })
defineProps<{ allowed: boolean; reason?: string; disabled?: boolean }>()
</script>

<style scoped>
.guarded-btn { display: inline-flex; }
.guarded-btn + .guarded-btn, .guarded-btn + .el-button, .el-button + .guarded-btn { margin-left: 12px; }
</style>
