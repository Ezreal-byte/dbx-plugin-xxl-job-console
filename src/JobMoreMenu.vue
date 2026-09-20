<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { ChevronDown, Copy, ListTree, Pencil, Trash2 } from '@lucide/vue';
import { t } from './i18n';

defineProps<{ disabled: boolean }>();
const emit = defineEmits<(event: 'nodes' | 'edit' | 'delete' | 'copy') => void>();
const trigger = ref<HTMLElement>(), menu = ref<HTMLElement>();
const open = ref(false), style = ref<Record<string, string>>({});
function close() { open.value = false; }
function toggle() {
  if (open.value) { close(); return; }
  const rect = trigger.value?.getBoundingClientRect(); if (!rect) return;
  const width = 150, height = 148;
  style.value = { left: `${Math.max(8, Math.min(rect.right - width, window.innerWidth - width - 8))}px`, top: `${rect.bottom + height > window.innerHeight ? Math.max(8, rect.top - height - 4) : rect.bottom + 4}px` };
  open.value = true;
  void nextTick(() => menu.value?.querySelector<HTMLElement>('button')?.focus());
}
function choose(action: 'nodes' | 'edit' | 'delete' | 'copy') { close(); emit(action); }
function outside(event: PointerEvent) { if (!trigger.value?.contains(event.target as Node) && !menu.value?.contains(event.target as Node)) close(); }
function scroll(event: Event) { if (!menu.value?.contains(event.target as Node)) close(); }
onMounted(() => { document.addEventListener('pointerdown', outside, true); document.addEventListener('scroll', scroll, true); window.addEventListener('resize', close); });
onBeforeUnmount(() => { document.removeEventListener('pointerdown', outside, true); document.removeEventListener('scroll', scroll, true); window.removeEventListener('resize', close); });
</script>
<template>
  <div class="job-more"><button ref="trigger" class="row-action" type="button" :aria-expanded="open" :aria-label="t('更多')" @click="toggle" @keydown.esc="close">{{ t('更多') }}<ChevronDown :size="13" /></button>
    <Teleport to="body"><div v-if="open" ref="menu" class="job-more-menu" :style="style" role="menu" @keydown.esc="close"><button role="menuitem" @click="choose('nodes')"><ListTree :size="15" />{{ t('注册节点') }}</button><button role="menuitem" :disabled="disabled" @click="choose('edit')"><Pencil :size="15" />{{ t('编辑') }}</button><button role="menuitem" :disabled="disabled" @click="choose('copy')"><Copy :size="15" />{{ t('复制') }}</button><button role="menuitem" class="danger-text" :disabled="disabled" @click="choose('delete')"><Trash2 :size="15" />{{ t('删除') }}</button></div></Teleport>
  </div>
</template>
