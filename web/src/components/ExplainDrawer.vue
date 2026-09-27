<template>
  <el-drawer :model-value="modelValue" :title="drawerTitle" size="440px" class="explain-drawer" append-to-body
    @update:model-value="(v: boolean) => emit('update:modelValue', v)" @open="load">
    <div v-if="loading" class="ex-muted">{{ t('explain.loading') }}</div>
    <div v-else-if="error" class="ex-error">{{ error }}</div>
    <div v-else-if="ex" class="ex-body" data-test="explain-body">
      <section class="ex-head">
        <el-tag :type="verdictType(ex.verdict)" effect="dark" size="large" class="ex-verdict">{{ ex.conclusion }}</el-tag>
        <div class="ex-meta">
          <span>{{ t('explain.due', { date: ex.due_date }) }}</span>
          <span v-if="ex.planned_finish">{{ t('explain.planned', { date: ex.planned_finish }) }}</span>
          <span class="ex-conf" :class="'conf-' + ex.confidence">
            {{ t('explain.confidence') }}：{{ t(confidenceKey(ex.confidence)) }}
          </span>
        </div>
      </section>

      <section>
        <h4>{{ t('explain.evidence') }}</h4>
        <div v-if="!ex.evidence.length" class="ex-muted">{{ t('explain.noEvidence') }}</div>
        <ul v-else class="ex-evidence">
          <li v-for="(e, i) in ex.evidence" :key="i" :data-ref="e.ref">
            <span class="ex-ev-type" :class="'ev-' + e.type">{{ t(evidenceKey(e.type)) }}</span>
            <span class="ex-ev-text">{{ e.text }}</span>
          </li>
        </ul>
      </section>

      <section v-if="ex.missing.length" class="ex-missing">
        <h4>{{ t('explain.missing') }}</h4>
        <ul>
          <li v-for="(m, i) in ex.missing" :key="i">{{ m }}</li>
        </ul>
      </section>

      <section class="ex-suggest">
        <h4>{{ t('explain.suggestion') }}</h4>
        <p>{{ ex.suggestion }}</p>
        <el-button v-if="ex.preview" type="primary" plain size="small" data-test="explain-preview"
          @click="emit('preview', ex.preview)">
          {{ t('explain.openPreview') }}
        </el-button>
      </section>

      <section v-if="ex.assumptions.length" class="ex-assume">
        <h4>{{ t('explain.assumptions') }}</h4>
        <ul>
          <li v-for="(a, i) in ex.assumptions" :key="i">{{ a }}</li>
        </ul>
      </section>

      <p class="ex-note">{{ t('explain.ruleNote') }}</p>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import teamApi from '@/utils/team-api'
import { confidenceKey, evidenceKey, verdictType, type Explanation, type SuggestedPreview } from '@/utils/explain'

const props = defineProps<{ modelValue: boolean; deadlineId: string; orderNo?: string }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'preview', p: SuggestedPreview): void
}>()
const { t } = useI18n()

const ex = ref<Explanation | null>(null)
const loading = ref(false)
const error = ref('')

const drawerTitle = computed(() => props.orderNo ? `${t('explain.title')} · ${props.orderNo}` : t('explain.title'))

async function load() {
  if (!props.deadlineId) return
  loading.value = true
  error.value = ''
  ex.value = null
  try {
    const { data } = await teamApi.get(`/deadlines/${props.deadlineId}/explain`)
    ex.value = (data?.data ?? data) as Explanation
  } catch (e: any) {
    error.value = e?.response?.data?.error || t('explain.failed')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.ex-body { display: flex; flex-direction: column; gap: 18px; }
.ex-body h4 { margin: 0 0 8px; font-size: 13px; font-weight: 600; color: var(--md-text-dim); letter-spacing: 0.02em; }
.ex-head { display: flex; flex-direction: column; gap: 10px; }
.ex-verdict { height: auto; white-space: normal; line-height: 1.5; padding: 8px 12px; font-size: 14px; text-align: left; }
.ex-meta { display: flex; flex-wrap: wrap; gap: 12px; font-size: 12px; color: var(--md-text-dim); }
.ex-conf { font-weight: 600; }
.ex-conf.conf-high { color: var(--el-color-success); }
.ex-conf.conf-medium { color: var(--el-color-warning); }
.ex-conf.conf-low { color: var(--el-color-danger); }
.ex-evidence { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.ex-evidence li { display: flex; gap: 8px; align-items: flex-start; font-size: 13px; line-height: 1.55; color: var(--md-text); }
.ex-ev-type {
  flex: none; font-size: 11px; padding: 1px 6px; border-radius: 4px; margin-top: 2px;
  background: var(--md-surface-2, rgba(127,127,127,.15)); color: var(--md-text-dim);
}
.ex-ev-type.ev-event { color: var(--el-color-primary); }
.ex-ev-type.ev-load, .ex-ev-type.ev-plan { color: var(--el-color-warning); }
.ex-ev-type.ev-history, .ex-ev-type.ev-flag { color: var(--el-color-danger); }
.ex-ev-type.ev-progress { color: var(--el-color-success); }
.ex-missing ul, .ex-assume ul { margin: 0; padding-left: 18px; font-size: 13px; line-height: 1.6; }
.ex-missing { padding: 10px 12px; border-radius: 8px; border: 1px dashed var(--el-color-warning); }
.ex-missing li { color: var(--el-color-warning); }
.ex-suggest p { margin: 0 0 8px; font-size: 13px; line-height: 1.6; color: var(--md-text); }
.ex-assume li { color: var(--md-text-dim); }
.ex-note { margin: 0; font-size: 12px; color: var(--md-text-dim); }
.ex-muted { color: var(--md-text-dim); font-size: 13px; }
.ex-error { color: var(--el-color-danger); font-size: 13px; }
</style>
