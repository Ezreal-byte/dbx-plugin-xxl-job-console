<template><div class="loading-skeleton" role="status" :aria-label="t('读取中')"><div v-for="n in 6" :key="n"><i /><i /><i /></div><span class="sr-only">{{ t('读取中') }}</span></div></template>
<script setup lang="ts">import { t } from './i18n';</script>
