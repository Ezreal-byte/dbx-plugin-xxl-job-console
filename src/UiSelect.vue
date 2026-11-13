<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue';
import { Check, ChevronDown, Search } from '@lucide/vue';
import { t } from './i18n';
import { matchesOption } from './select';

export type SelectOption = { value: string | number; label: string; disabled?: boolean; keywords?: string };
const props = defineProps<{ modelValue: string | number; options: SelectOption[]; disabled?: boolean; placeholder?: string; ariaLabel?: string; searchable?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: string | number]; change: [value: string | number] }>();
const trigger = ref<HTMLElement>(), menu = ref<HTMLElement>(), searchInput = ref<HTMLInputElement>();
const opened = ref(false), active = ref(-1), query = ref(''), menuStyle = ref<Record<string, string>>({});
const id = useId();
const selected = computed(() => props.options.find(option => option.value === props.modelValue));
const filtered = computed(() => props.options.map((option, index) => ({ option, index })).filter(({ option }) => matchesOption(option, query.value)));
const enabled = computed(() => filtered.value.filter(({ option }) => !option.disabled));
function position() {
  const rect = trigger.value?.getBoundingClientRect(); if (!rect) return;
  const height = Math.min(280, props.options.length * 37 + 10 + (props.searchable ? 44 : 0));
  const below = window.innerHeight - rect.bottom;
  const top = below >= Math.min(height, 180) || below >= rect.top ? rect.bottom + 5 : Math.max(8, rect.top - height - 5);
  menuStyle.value = { top: `${top}px`, left: `${Math.max(8, Math.min(rect.left, window.innerWidth - rect.width - 8))}px`, minWidth: `${rect.width}px`, maxWidth: `${Math.min(360, window.innerWidth - 16)}px`, maxHeight: `${Math.max(110, Math.min(280, window.innerHeight - top - 10))}px` };
}
function scrollActive() { void nextTick(() => menu.value?.querySelector<HTMLElement>(`[data-index="${active.value}"]`)?.scrollIntoView({ block: 'nearest' })); }
function open(prefill = '') {
  if (props.disabled || opened.value) return;
  query.value = prefill;
  active.value = filtered.value.find(({ option }) => option.value === props.modelValue && !option.disabled)?.index ?? enabled.value[0]?.index ?? -1;
  position(); opened.value = true;
  void nextTick(() => props.searchable ? searchInput.value?.focus() : menu.value?.querySelector<HTMLElement>(`[data-index="${active.value}"]`)?.scrollIntoView({ block: 'nearest' }));
}
function close() { opened.value = false; query.value = ''; }
function choose(index: number) {
  const option = props.options[index]; if (!option || option.disabled) return;
  emit('update:modelValue', option.value); emit('change', option.value); close(); trigger.value?.focus();
}
function moveActive(delta: number | 'first' | 'last') {
  const positions = enabled.value.map(({ index }) => index);
  if (!positions.length) { active.value = -1; return; }
  if (delta === 'first') active.value = positions[0];
  else if (delta === 'last') active.value = positions[positions.length - 1];
  else { const current = positions.indexOf(active.value); active.value = current < 0 ? positions[delta > 0 ? 0 : positions.length - 1] : positions[(current + delta + positions.length) % positions.length]; }
  scrollActive();
}
function keydown(event: KeyboardEvent) {
  if (props.disabled) return;
  if (event.key === 'Escape') { if (opened.value) { event.preventDefault(); close(); } return; }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Home' || event.key === 'End') {
    event.preventDefault(); if (!opened.value) { open(); return; }
    moveActive(event.key === 'Home' ? 'first' : event.key === 'End' ? 'last' : event.key === 'ArrowDown' ? 1 : -1);
    return;
  }
  if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); if (opened.value) choose(active.value); else open(); }
  else if (props.searchable && event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) { event.preventDefault(); open(event.key); }
}
function searchKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); trigger.value?.focus(); return; }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Home' || event.key === 'End') {
    event.preventDefault();
    moveActive(event.key === 'Home' ? 'first' : event.key === 'End' ? 'last' : event.key === 'ArrowDown' ? 1 : -1);
    return;
  }
  if (event.key === 'Enter') { event.preventDefault(); if (active.value >= 0) choose(active.value); }
  else if (event.key === 'Tab') close();
}
function outside(event: PointerEvent) { if (!trigger.value?.contains(event.target as Node) && !menu.value?.contains(event.target as Node)) close(); }
function scrolling(event: Event) { if (opened.value && !menu.value?.contains(event.target as Node)) close(); }
onMounted(() => { document.addEventListener('pointerdown', outside, true); document.addEventListener('scroll', scrolling, true); window.addEventListener('resize', close); });
onBeforeUnmount(() => { document.removeEventListener('pointerdown', outside, true); document.removeEventListener('scroll', scrolling, true); window.removeEventListener('resize', close); });
watch(() => props.disabled, value => { if (value) close(); });
watch(query, () => { active.value = filtered.value.find(({ option }) => option.value === props.modelValue && !option.disabled)?.index ?? enabled.value[0]?.index ?? -1; scrollActive(); });
</script>
<template>
  <div class="ui-select" :class="{ 'is-open': opened, 'is-disabled': disabled }">
    <button ref="trigger" class="ui-select-trigger" type="button" role="combobox" :aria-label="ariaLabel" aria-haspopup="listbox" :aria-expanded="opened" :aria-controls="id" :aria-activedescendant="opened && active >= 0 ? `${id}-${active}` : undefined" :disabled="disabled" @click="opened ? close() : open()" @keydown="keydown"><span :class="{ 'is-placeholder': !selected }">{{ selected?.label || placeholder || '—' }}</span><ChevronDown :size="15" /></button>
    <Teleport to="body"><div v-if="opened" ref="menu" class="ui-select-menu" :style="menuStyle" @keydown="searchKeydown"><div v-if="searchable" class="ui-select-search"><Search :size="14" /><input ref="searchInput" v-model="query" type="search" :placeholder="t('搜索选项')" :aria-label="t('搜索选项')" :aria-controls="id" :aria-activedescendant="active >= 0 ? `${id}-${active}` : undefined" autocomplete="off" spellcheck="false" /></div><div :id="id" role="listbox" :aria-label="ariaLabel"><button v-for="{ option, index } in filtered" :id="`${id}-${index}`" :key="`${option.value}-${index}`" class="ui-select-option" type="button" role="option" :data-index="index" :aria-selected="option.value === modelValue" :class="{ active: index === active, selected: option.value === modelValue }" :disabled="option.disabled" @pointerenter="!option.disabled && (active = index)" @click="choose(index)"><span>{{ option.label }}</span><Check v-if="option.value === modelValue" :size="15" /></button><p v-if="!filtered.length" class="ui-select-empty" role="status">{{ t('无匹配项') }}</p></div></div></Teleport>
  </div>
</template>
