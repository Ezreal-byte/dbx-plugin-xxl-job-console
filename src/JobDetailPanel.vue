<script setup lang="ts">
import { Sparkles, FileText, RefreshCw, X } from '@lucide/vue';
import { type JobDetail, type LogRow, jobStatus, timeLabel } from './domain';
import { t } from './i18n';
import LoadingSkeleton from './LoadingSkeleton.vue';
defineProps<{ detail?: JobDetail; busy: boolean; error: string }>();
defineEmits<{ close: []; refresh: []; ai: []; log: [row: LogRow] }>();
function executionStatus(row: LogRow) { return !row.triggerCode ? '等待调度' : row.triggerCode !== 200 ? '调度失败' : row.handleCode === 200 ? '执行成功' : !row.handleCode ? '执行中' : '执行失败'; }
</script>
<template>
  <div class="overlay" @keydown.esc.stop="$emit('close')"><section class="modal job-detail-panel" role="dialog" aria-modal="true" :aria-label="t('任务详情')">
    <header><div><small>{{ t('任务详情') }} #{{ detail?.job.id }}</small><h2>{{ detail?.job.jobDesc || t('读取中') }}</h2></div><button class="icon" :aria-label="t('关闭')" @click="$emit('close')"><X :size="18" /></button></header>
    <div class="modal-body"><p v-if="error" class="banner error" role="alert">{{ error }}</p><LoadingSkeleton v-if="busy" />
      <template v-else-if="detail">
        <div class="detail-summary"><span class="badge" :class="jobStatus(detail.job.triggerStatus).tone">{{ t(jobStatus(detail.job.triggerStatus).label) }}</span><code>{{ detail.job.executorHandler || detail.job.glueType }}</code><span>{{ detail.executor.title }}</span></div>
        <div class="detail-grid"><article><h3>{{ t('调度配置') }}</h3><dl><dt>{{ t('调度类型') }}</dt><dd>{{ detail.job.scheduleType }}</dd><dt>Cron / {{ t('间隔（秒）') }}</dt><dd><code>{{ detail.job.scheduleConf || '—' }}</code></dd><dt>{{ t('调度过期策略') }}</dt><dd>{{ detail.job.misfireStrategy }}</dd><dt>{{ t('子任务 ID') }}</dt><dd>{{ detail.job.childJobId || '—' }}</dd></dl></article><article><h3>{{ t('执行策略') }}</h3><dl><dt>{{ t('路由策略') }}</dt><dd>{{ detail.job.executorRouteStrategy }}</dd><dt>{{ t('阻塞策略') }}</dt><dd>{{ detail.job.executorBlockStrategy }}</dd><dt>{{ t('超时（秒）') }}</dt><dd>{{ detail.job.executorTimeout }}</dd><dt>{{ t('失败重试次数') }}</dt><dd>{{ detail.job.executorFailRetryCount }}</dd></dl></article></div>
        <h3>{{ t('注册节点') }}</h3><ul class="registered-nodes"><li v-for="node in detail.nodes" :key="node"><code>{{ node }}</code></li></ul><p v-if="!detail.nodes.length" class="muted">{{ t('暂无注册节点') }}</p>
        <h3>{{ t('近期执行记录') }}</h3><p v-if="detail.warning" class="banner warning">{{ detail.warning }}</p><p v-if="!detail.recentLogs.length" class="muted">{{ t('最近24小时没有记录') }}</p>
        <div v-for="row in detail.recentLogs" :key="row.id" class="detail-log"><code>#{{ row.id }}</code><span>{{ timeLabel(row.triggerTime) }}</span><span class="badge" :class="row.handleCode === 200 ? 'success' : 'neutral'">{{ t(executionStatus(row)) }}</span><button @click="$emit('log', row)"><FileText :size="14" />{{ t('查看日志') }}</button></div>
      </template>
    </div>
    <footer><button :disabled="busy" @click="$emit('refresh')"><RefreshCw :size="15" />{{ t('刷新') }}</button><div class="spacer" /><button class="primary" :disabled="busy || !detail" @click="$emit('ai')"><Sparkles :size="15" />{{ t('AI 分析') }}</button><button @click="$emit('close')">{{ t('关闭') }}</button></footer>
  </section></div>
</template>
