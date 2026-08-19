<script setup lang="ts">
import { onBeforeUnmount } from 'vue'
import { NButton, NForm, NFormItem, NIcon, NModal, NSelect, NTag } from 'naive-ui'
import { Bot, Database, FolderPlus, MessageSquarePlus } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useChatStore } from '@/stores/chat'
import { useProjectStore } from '@/stores/project'
import { useVoiceChat } from '@/composables/useVoiceChat'
import ChatWorkbench from '@/components/ChatWorkbench.vue'

const workspace = useWorkspaceStore()
const ui = useUiStore()
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
  maxRows,
  chatDatasources
} = storeToRefs(chat)
const { projects, projectOptions, projectSelectable, selectedProject } = storeToRefs(projectStore)
const { voiceRecording, voiceBusy } = voice

onBeforeUnmount(() => {
  voice.cancelVoiceInput()
})
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
        <div v-if="!projects.total" class="project-empty-guide">
          <div class="project-empty-mark">
            <NIcon :component="Database" />
          </div>
          <div class="project-empty-copy">
            <span>项目未就绪</span>
            <h3>先创建项目，再开始问数</h3>
            <p>项目会承载数据源、业务知识和成员权限。</p>
          </div>
          <div class="project-empty-actions">
            <NButton type="primary" secondary @click="ui.activeModule = 'project'">
              <template #icon>
                <NIcon :component="FolderPlus" />
              </template>
              去创建项目
            </NButton>
          </div>
        </div>
      </section>

      <ChatWorkbench
        v-else
        :messages="chat.messages"
        :datasources="chatDatasources"
        :session-id="selectedSession.id"
        :project-name="selectedProject?.name || '未选择项目'"
        :session-title="selectedSession.title"
        :auto-execute="true"
        v-model:max-rows="maxRows"
        :loading="loading"
        :voice-recording="voiceRecording"
        :voice-busy="voiceBusy"
        :voice-enabled="true"
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
