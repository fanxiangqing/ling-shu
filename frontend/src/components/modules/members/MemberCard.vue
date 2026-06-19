<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon, NPopconfirm, NTag } from 'naive-ui'
import { Power, PowerOff, Trash2 } from '@lucide/vue'
import type { MemberRecord } from '@/types/domain'
import { memberAccountName, memberDisplayName, memberStatus, memberStatusLabel, memberStatusTagType } from '@/utils/format'

const props = defineProps<{
  member: MemberRecord
  scope: 'tenant' | 'project'
}>()

const emit = defineEmits<{
  toggleStatus: [member: MemberRecord]
  delete: [member: MemberRecord]
}>()

const isActive = computed(() => memberStatus(props.member) === 'active')
const scopeLabel = computed(() => props.scope === 'tenant' ? '组织' : '项目')
const displayName = computed(() => memberDisplayName(props.member))
const accountName = computed(() => memberAccountName(props.member))
const toggleLabel = computed(() => isActive.value ? '停用' : '启用')
const toggleIcon = computed(() => isActive.value ? PowerOff : Power)
const toggleType = computed(() => isActive.value ? 'warning' : 'success')
</script>

<template>
  <article class="member-card">
    <div class="member-avatar">{{ displayName.slice(0, 1).toUpperCase() }}</div>
    <div class="member-card-main">
      <strong>{{ displayName }}</strong>
      <span>{{ accountName }}</span>
    </div>
    <div class="member-card-controls">
      <NTag size="small" :type="memberStatusTagType(member)">{{ memberStatusLabel(member) }}</NTag>
      <div class="member-card-actions">
        <NPopconfirm @positive-click="emit('toggleStatus', member)">
          <template #trigger>
            <NButton size="small" quaternary :type="toggleType">
              <template #icon>
                <NIcon :component="toggleIcon" />
              </template>
              {{ toggleLabel }}
            </NButton>
          </template>
          {{ toggleLabel }}后会{{ isActive ? '暂停' : '恢复' }}该成员的{{ scopeLabel }}访问权限，确定继续吗？
        </NPopconfirm>
        <NPopconfirm @positive-click="emit('delete', member)">
          <template #trigger>
            <NButton size="small" quaternary type="error">
              <template #icon>
                <NIcon :component="Trash2" />
              </template>
              {{ scope === 'tenant' ? '删除' : '移除' }}
            </NButton>
          </template>
          {{ scope === 'tenant' ? '删除组织成员会同步移除其项目成员关系' : '移除后该成员将无法进入当前项目' }}，确定继续吗？
        </NPopconfirm>
      </div>
    </div>
  </article>
</template>
