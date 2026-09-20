<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch, nextTick } from 'vue';
import * as echarts from 'echarts/core';
import { LineChart, PieChart } from 'echarts/charts';
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { t } from './i18n';

echarts.use([LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer]);
export type ReportData = { triggerDayList: string[]; triggerDayCountRunningList: number[]; triggerDayCountSucList: number[]; triggerDayCountFailList: number[] };
const props = defineProps<{ report?: ReportData; appearance: number }>();
const trendEl = ref<HTMLElement>(), shareEl = ref<HTMLElement>();
let trend: echarts.ECharts | undefined, share: echarts.ECharts | undefined, observer: ResizeObserver | undefined;
const css = (name: string, fallback: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
function draw() {
  if (!trendEl.value || !shareEl.value) return;
  trend ||= echarts.init(trendEl.value);
  share ||= echarts.init(shareEl.value);
  const report = props.report;
  const color = css('--color-foreground', '#24282d'), muted = css('--color-muted-foreground', '#717781'), line = css('--color-border', '#e1e4e7');
  const green = css('--color-chart-2', '#168465'), red = css('--color-destructive', '#d05757'), amber = css('--color-chart-3', '#d79e39');
  const sums = [report?.triggerDayCountSucList || [], report?.triggerDayCountFailList || [], report?.triggerDayCountRunningList || []].map(items => items.reduce((sum, n) => sum + (Number(n) || 0), 0));
  const common = { textStyle: { color, fontFamily: css('--dbx-plugin-ui-font-family', 'sans-serif') }, animationDuration: 320 };
  trend.setOption({ ...common, color: [green, red, amber], tooltip: { trigger: 'axis', axisPointer: { type: 'line', snap: true, lineStyle: { color: line, type: 'dashed' } }, backgroundColor: css('--color-card', '#fff'), borderColor: line, textStyle: { color } }, legend: { bottom: 0, textStyle: { color: muted } }, grid: { left: 38, right: 18, top: 18, bottom: 54 }, xAxis: { type: 'category', data: report?.triggerDayList || [], axisLabel: { color: muted }, axisLine: { lineStyle: { color: line } } }, yAxis: { type: 'value', minInterval: 1, axisLabel: { color: muted }, splitLine: { lineStyle: { color: line } } }, series: [[t('成功'), report?.triggerDayCountSucList], [t('失败'), report?.triggerDayCountFailList], [t('运行中'), report?.triggerDayCountRunningList]].map(([name, data]) => ({ name, type: 'line', smooth: true, symbolSize: 6, data, lineStyle: { width: 2.5 }, areaStyle: { opacity: .07 }, emphasis: { disabled: true }, blur: { lineStyle: { opacity: 1 }, itemStyle: { opacity: 1 } } })) }, true);
  share.setOption({ ...common, color: [green, red, amber], tooltip: { trigger: 'item', backgroundColor: css('--color-card', '#fff'), borderColor: line, textStyle: { color } }, legend: { bottom: 0, textStyle: { color: muted } }, series: [{ type: 'pie', radius: ['50%', '72%'], center: ['50%', '45%'], avoidLabelOverlap: true, label: { color, formatter: '{b}\n{d}%' }, emphasis: { disabled: true }, blur: { itemStyle: { opacity: 1 }, label: { opacity: 1 } }, data: [[t('成功'), sums[0]], [t('失败'), sums[1]], [t('运行中'), sums[2]]].map(([name, value]) => ({ name, value })) }] }, true);
  trend.resize(); share.resize();
}
onMounted(async () => { await nextTick(); observer = new ResizeObserver(() => { trend?.resize(); share?.resize(); }); if (trendEl.value) observer.observe(trendEl.value); if (shareEl.value) observer.observe(shareEl.value); draw(); });
watch(() => [props.report, props.appearance], () => nextTick(draw), { deep: true });
onBeforeUnmount(() => { observer?.disconnect(); trend?.dispose(); share?.dispose(); });
</script>
<template><div class="report-plots"><article class="report-plot"><div class="plot-heading"><h2>{{ t('调度趋势') }}</h2><span>{{ t('按日统计') }}</span></div><div ref="trendEl" class="chart-canvas" role="img" :aria-label="t('调度趋势')" /></article><article class="report-plot"><div class="plot-heading"><h2>{{ t('调度结果占比') }}</h2><span>{{ t('所选时段') }}</span></div><div ref="shareEl" class="chart-canvas" role="img" :aria-label="t('调度结果占比')" /></article></div></template>
