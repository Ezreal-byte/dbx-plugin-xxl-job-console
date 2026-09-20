<script setup lang="ts">
import { ref, computed } from 'vue';
import { X, Save } from '@lucide/vue';
import { type Group, validateGroup } from './domain';
import { t } from './i18n';
const props = defineProps<{ group: Group; busy: boolean; serverError?: string }>();
const emit = defineEmits<{ save: [group: Group]; close: [] }>();
const original = JSON.stringify(props.group);
const form = ref<Group>({ ...props.group });
const error = ref('');
defineExpose({ isDirty: computed(() => JSON.stringify(form.value) !== original) });
function save() { error.value = t(validateGroup(form.value)); if (!error.value) emit('save', structuredClone({ ...form.value })); }
</script>
<template>
  <section class="modal editor-dialog" role="dialog" aria-modal="true" :aria-label="t(group.id ? '编辑执行器' : '新增执行器')">
    <header><div><small v-if="group.id">{{ t('执行器') }} #{{ group.id }}</small><h2>{{ t(group.id ? '编辑执行器' : '新增执行器') }}</h2></div><button class="icon" :aria-label="t('关闭')" :title="t('关闭')" :disabled="busy" @click="emit('close')"><X :size="18" /></button></header>
    <div class="editor-form"><div class="form-body">
      <p v-if="error || serverError" class="error" role="alert">{{ error || serverError }}</p>
      <label>AppName<input v-model="form.appname" required minlength="4" maxlength="64" :readonly="!!group.id" :disabled="busy" class="mono" /></label>
      <label>{{ t('执行器名称') }}<input v-model="form.title" required maxlength="64" :disabled="busy" /></label>
      <fieldset><legend>{{ t('注册方式') }}</legend><label class="inline"><input v-model.number="form.addressType" type="radio" :value="0" :disabled="busy" />{{ t('自动注册') }}</label><label class="inline"><input v-model.number="form.addressType" type="radio" :value="1" :disabled="busy" />{{ t('手动录入') }}</label></fieldset>
      <label>{{ t('执行器地址') }}<textarea v-model="form.addressList" rows="6" :readonly="form.addressType === 0" :required="form.addressType === 1" :disabled="busy" class="mono" spellcheck="false" /></label>
    </div><footer><button type="button" :disabled="busy" @click="emit('close')">{{ t('取消') }}</button><button class="primary" type="button" :disabled="busy" @click="save"><Save :size="16" />{{ t('保存') }}</button></footer></div>
  </section>
</template>
