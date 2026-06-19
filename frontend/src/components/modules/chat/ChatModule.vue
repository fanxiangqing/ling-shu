<script setup lang="ts">
import { NButton, NForm, NFormItem, NIcon, NModal, NSelect, NTag } from 'naive-ui'
import { Bot, Database, MessageSquarePlus } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useChatStore } from '@/stores/chat'
import { useProjectStore } from '@/stores/project'
import { useVoiceChat } from '@/composables/useVoiceChat'
import ChatWorkbench from '@/components/ChatWorkbench.vue'

const workspace = useWorkspaceStore()
const chat = useChatStore()
const projectStore = useProjectStore()
const voice = useVoiceChat()

const { loading } = storeToRefs(workspace)
const {
  sessions,
  visibleSessions,
  selectedSession,
  sessionLoadingMore,
  chatProjectModalVisible,
  chatForm,
  autoExecute,
  maxRows,
  chatDatasources
} = storeToRefs(chat)
const { projects, projectOptions, projectSelectable, selectedProject } = storeToRefs(projectStore)
const { voiceRecording, voiceBusy } = voice
</script>

<template>
  <section class="module-page chat-page">
    <div class="chatgpt-layout">
      <aside class="chat-history-panel chatgpt-sidebar">
        <NButton block type="primary" :disabled="!projects.total" @click="chat.openNewChatModal">
          <template #icon>
            <NIcon :component="MessageSquarePlus" />
          </template>
          新建对话
        </NButton>
        <div class="surface-head">
          <h2>历史对话</h2>
          <NTag size="small">{{ visibleSessions.length }} / {{ sessions.total }} 个</NTag>
        </div>
        <div class="chat-session-list" @scroll="chat.handleSessionListScroll">
          <button
            v-for="session in visibleSessions"
            :key="session.id"
            class="session-card"
            :class="{ active: session.id === workspace.context.sessionId }"
            type="button"
            @click="chat.enterSession(session)"
          >
            <strong>{{ session.title || '未命名会话' }}</strong>
            <span>{{ chat.sessionProjectName(session) }}</span>
          </button>
          <div v-if="sessionLoadingMore" class="session-scroll-hint">加载更多会话...</div>
        </div>
        <div v-if="!visibleSessions.length" class="mini-empty">暂无会话</div>
        <NButton block secondary @click="() => chat.refreshSessions()">刷新会话</NButton>
      </aside>

      <section v-if="!selectedSession" class="ask-home chatgpt-empty">
        <div class="bot-badge">
          <NIcon :component="Bot" />
        </div>
        <h2>你好，我是 Ling-Shu</h2>
        <p>用自然语言直接提问业务数据。每个对话会绑定一个项目，并继承这个项目的数据源、业务知识和权限范围。</p>
        <NButton type="primary" :disabled="!projects.total" @click="chat.openNewChatModal">
          <template #icon>
            <NIcon :component="MessageSquarePlus" />
          </template>
          新建对话
        </NButton>
        <div v-if="!projects.total" class="empty-state compact">
          <NIcon :component="Database" />
          <h2>还没有项目</h2>
          <p>已有项目会显示在这里；选择一个项目后就可以进入自然语言问数。</p>
          <NSelect
            :value="workspace.context.projectId || null"
            :options="projectOptions"
            :disabled="!projectSelectable"
            filterable
            placeholder="选择项目"
            @update:value="workspace.handleProjectChange"
          />
        </div>
      </section>

      <ChatWorkbench
        v-else
        :messages="chat.messages"
        :datasources="chatDatasources"
        :session-id="selectedSession.id"
        :project-name="selectedProject?.name || '未选择项目'"
        :session-title="selectedSession.title"
        v-model:auto-execute="autoExecute"
        v-model:max-rows="maxRows"
        :loading="loading"
        :voice-recording="voiceRecording"
        :voice-busy="voiceBusy"
        @ask="chat.ask"
        @voice-toggle="voice.toggleVoiceInput"
      />
    </div>

    <NModal
      v-model:show="chatProjectModalVisible"
      preset="card"
      title="新建对话"
      class="chat-project-modal"
      :mask-closable="false"
    >
      <NForm label-placement="top">
        <NFormItem label="选择项目">
          <NSelect
            v-model:value="chatForm.project_id"
            :options="projectOptions"
            filterable
            placeholder="选择要提问的项目"
          />
        </NFormItem>
        <div class="modal-actions">
          <NButton @click="chatProjectModalVisible = false">取消</NButton>
          <NButton type="primary" :loading="loading" @click="chat.createSession">
            <template #icon>
              <NIcon :component="MessageSquarePlus" />
            </template>
            开始对话
          </NButton>
        </div>
      </NForm>
    </NModal>
  </section>
</template>
