<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue';
import { Check, ChevronDown } from '@lucide/vue';

export type SelectOption = { value: string | number; label: string; disabled?: boolean };
const props = defineProps<{ modelValue: string | number; options: SelectOption[]; disabled?: boolean; placeholder?: string; ariaLabel?: string }>();
const emit = defineEmits<{ 'update:modelValue': [value: string | number]; change: [value: string | number] }>();
const trigger = ref<HTMLElement>(), menu = ref<HTMLElement>();
const opened = ref(false), active = ref(-1), menuStyle = ref<Record<string, string>>({});
const id = useId();
const selected = computed(() => props.options.find(option => option.value === props.modelValue));
const enabled = computed(() => props.options.map((option, index) => ({ option, index })).filter(({ option }) => !option.disabled));
function position() {
  const rect = trigger.value?.getBoundingClientRect(); if (!rect) return;
  const height = Math.min(280, props.options.length * 37 + 10);
  const below = window.innerHeight - rect.bottom;
  const top = below >= Math.min(height, 180) || below >= rect.top ? rect.bottom + 5 : Math.max(8, rect.top - height - 5);
  menuStyle.value = { top: `${top}px`, left: `${Math.max(8, Math.min(rect.left, window.innerWidth - rect.width - 8))}px`, minWidth: `${rect.width}px`, maxWidth: `${Math.min(360, window.innerWidth - 16)}px`, maxHeight: `${Math.max(110, Math.min(280, window.innerHeight - top - 10))}px` };
}
function open() {
  if (props.disabled || opened.value) return;
  active.value = props.options.findIndex(option => option.value === props.modelValue && !option.disabled);
  if (active.value < 0) active.value = enabled.value[0]?.index ?? -1;
  position(); opened.value = true;
  void nextTick(() => menu.value?.querySelector<HTMLElement>(`[data-index="${active.value}"]`)?.scrollIntoView({ block: 'nearest' }));
}
function close() { opened.value = false; }
function choose(index: number) {
  const option = props.options[index]; if (!option || option.disabled) return;
  emit('update:modelValue', option.value); emit('change', option.value); close(); trigger.value?.focus();
}
function keydown(event: KeyboardEvent) {
  if (props.disabled) return;
  if (event.key === 'Escape') { if (opened.value) { event.preventDefault(); close(); } return; }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Home' || event.key === 'End') {
    event.preventDefault(); if (!opened.value) { open(); return; }
    const positions = enabled.value.map(({ index }) => index);
    if (!positions.length) return;
    const current = positions.indexOf(active.value);
    active.value = event.key === 'Home' ? positions[0] : event.key === 'End' ? positions[positions.length - 1] : positions[(current + (event.key === 'ArrowDown' ? 1 : -1) + positions.length) % positions.length];
    void nextTick(() => menu.value?.querySelector<HTMLElement>(`[data-index="${active.value}"]`)?.scrollIntoView({ block: 'nearest' }));
    return;
  }
  if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); if (opened.value) choose(active.value); else open(); }
}
function outside(event: PointerEvent) { if (!trigger.value?.contains(event.target as Node) && !menu.value?.contains(event.target as Node)) close(); }
function scrolling(event: Event) { if (opened.value && !menu.value?.contains(event.target as Node)) close(); }
onMounted(() => { document.addEventListener('pointerdown', outside, true); document.addEventListener('scroll', scrolling, true); window.addEventListener('resize', close); });
onBeforeUnmount(() => { document.removeEventListener('pointerdown', outside, true); document.removeEventListener('scroll', scrolling, true); window.removeEventListener('resize', close); });
watch(() => props.disabled, value => { if (value) close(); });
</script>
<template>
  <div class="ui-select" :class="{ 'is-open': opened, 'is-disabled': disabled }">
    <button ref="trigger" class="ui-select-trigger" type="button" role="combobox" :aria-label="ariaLabel" aria-haspopup="listbox" :aria-expanded="opened" :aria-controls="id" :aria-activedescendant="opened && active >= 0 ? `${id}-${active}` : undefined" :disabled="disabled" @click="opened ? close() : open()" @keydown="keydown"><span :class="{ 'is-placeholder': !selected }">{{ selected?.label || placeholder || '—' }}</span><ChevronDown :size="15" /></button>
    <Teleport to="body"><div v-if="opened" :id="id" ref="menu" class="ui-select-menu" :style="menuStyle" role="listbox" :aria-label="ariaLabel"><button v-for="(option, index) in options" :id="`${id}-${index}`" :key="`${option.value}-${index}`" class="ui-select-option" type="button" role="option" :data-index="index" :aria-selected="option.value === modelValue" :class="{ active: index === active, selected: option.value === modelValue }" :disabled="option.disabled" @pointerenter="active = index" @click="choose(index)"><span>{{ option.label }}</span><Check v-if="option.value === modelValue" :size="15" /></button></div></Teleport>
  </div>
</template>
