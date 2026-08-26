<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpin,
  NTabPane,
  NTabs,
  NTag,
  NTooltip
} from 'naive-ui'
import { Check, Edit3, Plus, RefreshCw, Trash2, X } from '@lucide/vue'
import { userMemoryApi } from '@/api/resources'
import { notify } from '@/composables/useNotify'
import type { SessionEpisodeRecord, UserMemoryRecord } from '@/types/domain'

const props = defineProps<{
  show: boolean
  tenantId: number
  projectId: number
  projectOptions: Array<{ label: string; value: number }>
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})
const loading = ref(false)
const saving = ref(false)
const memories = ref<UserMemoryRecord[]>([])
const episodes = ref<SessionEpisodeRecord[]>([])
const editorVisible = ref(false)
const editingId = ref(0)
const editingMemoryKey = ref('')
const selectedProjectId = ref(0)
const form = reactive<{
  kind: UserMemoryRecord['kind']
  content: string
  scope_project_id: number
}>({
  kind: 'preference',
  content: '',
  scope_project_id: 0
})

const kindOptions: Array<{
  label: string
  value: UserMemoryRecord['kind']
  description: string
}> = [
  { label: '偏好', value: 'preference', description: '回答方式、图表类型和展示习惯' },
  { label: '个人信息', value: 'profile', description: '稳定的身份、职责和关注领域' },
  { label: '约定', value: 'convention', description: '长期沿用的业务规则或沟通约定' },
  { label: '长期指令', value: 'instruction', description: '每次回答都应持续遵循的要求' },
  { label: '纠正', value: 'correction', description: '持续覆盖曾经出现的错误理解' }
]
const scopeOptions = computed(() => [
  { label: '所有项目', value: 0 },
  ...props.projectOptions
])
const selectedProjectName = computed(() =>
  props.projectOptions.find((item) => item.value === selectedProjectId.value)?.label || '所有项目'
)

watch([() => props.show, () => props.tenantId], ([show]) => {
  if (!show) return
  const initialProjectId = props.projectOptions.some((item) => item.value === props.projectId)
    ? props.projectId
    : 0
  if (selectedProjectId.value === initialProjectId) {
    void refresh()
  } else {
    selectedProjectId.value = initialProjectId
  }
})

watch(selectedProjectId, () => {
  if (props.show) void refresh()
})

async function refresh() {
  if (!props.tenantId || loading.value) {
    if (!props.tenantId) {
      memories.value = []
      episodes.value = []
    }
    return
  }
  loading.value = true
  try {
    const [memoryPage, episodeItems] = await Promise.all([
      userMemoryApi.list(selectedProjectId.value, props.tenantId, { page: 1, page_size: 100 }),
      selectedProjectId.value > 0
        ? userMemoryApi.listEpisodes(selectedProjectId.value, props.tenantId, 20)
        : Promise.resolve([])
    ])
    memories.value = memoryPage.items
    episodes.value = episodeItems
  } catch (error) {
    notify.error(error instanceof Error ? error.message : '加载长期记忆失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = 0
  editingMemoryKey.value = ''
  form.kind = 'preference'
  form.content = ''
  form.scope_project_id = selectedProjectId.value
  editorVisible.value = true
}

function openEdit(item: UserMemoryRecord) {
  editingId.value = item.id
  editingMemoryKey.value = item.memory_key || ''
  form.kind = item.kind
  form.content = item.content
  form.scope_project_id = item.scope.project_id
  editorVisible.value = true
}

async function save() {
  if (!form.content.trim()) return notify.warning('请输入要记住的内容')
  saving.value = true
  try {
    const targetProjectId = form.scope_project_id
    const payload = {
      tenant_id: props.tenantId,
      kind: form.kind,
      memory_key: editingMemoryKey.value || undefined,
      content: form.content.trim(),
      scope_level: targetProjectId > 0 ? 'project' as const : 'tenant' as const
    }
    if (editingId.value) {
      await userMemoryApi.update(targetProjectId, editingId.value, payload)
    } else {
      await userMemoryApi.create(targetProjectId, payload)
    }
    editorVisible.value = false
    notify.success(editingId.value ? '记忆已更新' : '记忆已保存')
    if (!editingId.value && selectedProjectId.value !== targetProjectId) {
      selectedProjectId.value = targetProjectId
    } else {
      await refresh()
    }
  } catch (error) {
    notify.error(error instanceof Error ? error.message : '保存长期记忆失败')
  } finally {
    saving.value = false
  }
}

async function confirm(item: UserMemoryRecord) {
  await mutate(() => userMemoryApi.confirm(item.scope.project_id, item.id, props.tenantId), '候选记忆已确认')
}

async function reject(item: UserMemoryRecord) {
  await mutate(() => userMemoryApi.reject(item.scope.project_id, item.id, props.tenantId), '候选记忆已忽略')
}

async function remove(item: UserMemoryRecord) {
  await mutate(() => userMemoryApi.delete(item.scope.project_id, item.id, props.tenantId), '记忆已删除')
}

async function clearSelectedScope() {
  await mutate(
    () => userMemoryApi.clear(selectedProjectId.value, props.tenantId, false),
    `${selectedProjectName.value}范围内的记忆已清空`
  )
}

async function mutate(operation: () => Promise<unknown>, message: string) {
  if (loading.value) return
  loading.value = true
  try {
    await operation()
    notify.success(message)
    loading.value = false
    await refresh()
  } catch (error) {
    notify.error(error instanceof Error ? error.message : '长期记忆操作失败')
    loading.value = false
  }
}

function kindLabel(kind: UserMemoryRecord['kind']) {
  return kindOptions.find((item) => item.value === kind)?.label || kind
}

function statusLabel(status: UserMemoryRecord['status']) {
  if (status === 'candidate') return '待确认'
  if (status === 'active') return '生效中'
  if (status === 'quarantined') return '已隔离'
  return '已停用'
}

function scopeLabel(item: UserMemoryRecord) {
  if (item.scope.project_id === 0) return '所有项目'
  return props.projectOptions.find((option) => option.value === item.scope.project_id)?.label ||
    `项目 #${item.scope.project_id}`
}

function formatTime(value?: string) {
  if (!value) return '尚未使用'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="card"
    title="我的记忆"
    class="user-memory-modal"
    closable
    :bordered="false"
  >
    <div class="memory-modal-toolbar">
      <NButton type="primary" size="small" :disabled="!tenantId" @click="openCreate">
        <template #icon><NIcon :component="Plus" /></template>
        添加记忆
      </NButton>
      <NTooltip trigger="hover">
        <template #trigger>
          <NButton quaternary circle size="small" :loading="loading" @click="refresh">
            <template #icon><NIcon :component="RefreshCw" /></template>
          </NButton>
        </template>
        刷新
      </NTooltip>
      <span class="memory-toolbar-spacer" />
      <NSelect
        v-model:value="selectedProjectId"
        class="memory-project-select"
        :options="scopeOptions"
        :disabled="loading"
      />
      <NPopconfirm @positive-click="clearSelectedScope">
        <template #trigger>
          <NButton size="small" tertiary type="error" :disabled="!tenantId">清空所选范围</NButton>
        </template>
        {{
          selectedProjectId > 0
            ? `确认删除“${selectedProjectName}”中的全部个人记忆？`
            : '确认删除适用于所有项目的个人记忆？'
        }}
      </NPopconfirm>
    </div>

    <NAlert v-if="tenantId && !selectedProjectId" type="info" :bordered="false" class="memory-scope-alert">
      正在查看适用于所有项目的长期记忆。
    </NAlert>

    <NSpin :show="loading">
      <NTabs type="line" size="large" animated>
          <NTabPane name="memories" :tab="`长期记忆 ${memories.length}`">
            <div v-if="memories.length" class="memory-list">
              <article v-for="item in memories" :key="item.id" class="memory-row">
                <div class="memory-row-main">
                  <div class="memory-tags">
                    <NTag size="small" :type="item.status === 'candidate' ? 'warning' : 'success'">
                      {{ statusLabel(item.status) }}
                    </NTag>
                    <NTag size="small">{{ kindLabel(item.kind) }}</NTag>
                    <NTag size="small" :bordered="false">{{ scopeLabel(item) }}</NTag>
                  </div>
                  <p>{{ item.content }}</p>
                  <span>最近使用：{{ formatTime(item.last_applied_at || item.last_recalled_at) }}</span>
                </div>
                <div class="memory-row-actions">
                  <NTooltip v-if="item.status === 'candidate'" trigger="hover">
                    <template #trigger>
                      <NButton circle quaternary type="primary" size="small" @click="confirm(item)">
                        <template #icon><NIcon :component="Check" /></template>
                      </NButton>
                    </template>
                    确认并启用
                  </NTooltip>
                  <NTooltip v-if="item.status === 'candidate'" trigger="hover">
                    <template #trigger>
                      <NButton circle quaternary size="small" @click="reject(item)">
                        <template #icon><NIcon :component="X" /></template>
                      </NButton>
                    </template>
                    忽略候选
                  </NTooltip>
                  <NTooltip trigger="hover">
                    <template #trigger>
                      <NButton circle quaternary size="small" @click="openEdit(item)">
                        <template #icon><NIcon :component="Edit3" /></template>
                      </NButton>
                    </template>
                    编辑
                  </NTooltip>
                  <NPopconfirm @positive-click="remove(item)">
                    <template #trigger>
                      <NButton circle quaternary type="error" size="small">
                        <template #icon><NIcon :component="Trash2" /></template>
                      </NButton>
                    </template>
                    确认删除这条记忆？
                  </NPopconfirm>
                </div>
              </article>
            </div>
            <NEmpty v-else description="还没有长期记忆" />
          </NTabPane>

          <NTabPane v-if="selectedProjectId > 0" name="episodes" :tab="`会话摘要 ${episodes.length}`">
            <div v-if="episodes.length" class="episode-list">
              <article v-for="episode in episodes" :key="episode.id" class="episode-row">
                <div>
                  <strong>会话 #{{ episode.session_id }}</strong>
                  <span>{{ formatTime(episode.updated_at) }}</span>
                </div>
                <p>{{ episode.summary }}</p>
                <div v-if="episode.topics?.length" class="memory-tags">
                  <NTag v-for="topic in episode.topics" :key="topic" size="small">{{ topic }}</NTag>
                </div>
              </article>
            </div>
            <NEmpty v-else description="还没有可复用的会话摘要" />
          </NTabPane>
      </NTabs>
    </NSpin>
  </NModal>

  <NModal v-model:show="editorVisible" preset="card" class="memory-editor-modal" :title="editingId ? '编辑记忆' : '添加记忆'">
    <NForm label-placement="top">
      <NFormItem label="类型">
        <NRadioGroup v-model:value="form.kind" class="memory-kind-picker">
          <div
            v-for="option in kindOptions"
            :key="option.value"
            class="memory-kind-option"
            :class="{ active: form.kind === option.value }"
            @click="form.kind = option.value"
          >
            <NRadio :value="option.value" />
            <div>
              <strong>{{ option.label }}</strong>
              <span>{{ option.description }}</span>
            </div>
          </div>
        </NRadioGroup>
      </NFormItem>
      <NFormItem label="适用范围">
        <NSelect v-model:value="form.scope_project_id" :options="scopeOptions" :disabled="editingId > 0" />
      </NFormItem>
      <NFormItem label="记忆内容">
        <NInput
          v-model:value="form.content"
          type="textarea"
          placeholder="请输入需要长期记住的内容"
          :autosize="{ minRows: 4, maxRows: 8 }"
        />
      </NFormItem>
      <div class="modal-actions">
        <NButton @click="editorVisible = false">取消</NButton>
        <NButton type="primary" :loading="saving" @click="save">保存</NButton>
      </div>
    </NForm>
  </NModal>
</template>
