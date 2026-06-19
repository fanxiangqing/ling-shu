<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag
} from 'naive-ui'
import { RotateCw, Settings2 } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { notify } from '@/composables/useNotify'
import { useProjectStore } from '@/stores/project'
import { useWorkspaceStore } from '@/stores/workspace'
import type { EmbedAppRecord } from '@/types/domain'

const project = useProjectStore()
const workspace = useWorkspaceStore()
const {
  projectEmbedModalVisible,
  embedApps,
  embedForm,
  lastCreatedEmbed,
  revealedEmbedSecret,
  embedProjectId
} = storeToRefs(project)

const selectedProject = computed(() => project.projectOptionRecords.find((item) => item.id === embedProjectId.value))
const origin = computed(() => window.location.origin)
const integrationTestVisible = ref(false)
const integrationTestConfigVisible = ref(false)
const integrationTestApp = ref<EmbedAppRecord | null>(null)
const integrationTestDoc = ref('')
const integrationTestExpiresAt = ref('')
const integrationTestForm = reactive({
  external_user_id: '',
  external_user_name: '控制台测试用户',
  key: 'dashboard:123',
  session_mode: 'new',
  parent_origin: ''
})

const sessionPolicyOptions = [
  { label: '按业务上下文复用', value: 'context' },
  { label: '按用户默认复用', value: 'user' },
  { label: '每次打开新会话', value: 'new' }
]

const sessionModeOptions = [
  { label: '新建测试会话', value: 'new' },
  { label: '复用测试会话', value: 'reuse' }
]

function integrationCode(app: EmbedAppRecord) {
  return `<script src="${origin.value}/sdk/ling-shu-embed.js"><\/script>
<script>
  LingShuEmbed.init({
    appId: "${app.app_id}",
    key: "dashboard:123",
    tokenProvider: () => fetch("/api/lingshu/embed-token").then((res) => res.json()),
    position: "bottom-right"
  })
<\/script>`
}

function tokenServerExample(app: EmbedAppRecord, secret = '只在创建后展示一次，请替换为你的真实 secret') {
  return `// 示例：第三方系统后端接口 /api/lingshu/embed-token
const resp = await fetch("${origin.value}/api/v1/embed/token", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    app_id: "${app.app_id}",
    app_secret: "${secret}",
    external_user_id: currentUser.id,
    external_user_name: currentUser.name,
    ttl_seconds: 3600
  })
})
const body = await resp.json()
return body.data.access_token`
}

function displayedSecret(app: EmbedAppRecord) {
  if (revealedEmbedSecret.value?.app_id === app.app_id) return revealedEmbedSecret.value.app_secret
  if (lastCreatedEmbed.value?.app.app_id === app.app_id) return lastCreatedEmbed.value.app_secret
  return ''
}

function policyLabel(value: string) {
  return sessionPolicyOptions.find((item) => item.value === value)?.label || value
}

function statusLabel(status?: string) {
  if (status === 'disabled') return '已停用'
  return '启用中'
}

function statusType(status?: string): 'success' | 'warning' {
  return status === 'disabled' ? 'warning' : 'success'
}

function allowedOrigins(app: EmbedAppRecord) {
  if (!app.allowed_origins_json) return []
  try {
    const values = JSON.parse(app.allowed_origins_json)
    return Array.isArray(values) ? values.filter((item) => typeof item === 'string' && item.trim()) : []
  } catch {
    return []
  }
}

function defaultParentOrigin(app: EmbedAppRecord) {
  return allowedOrigins(app)[0] || origin.value
}

function defaultTestUserID() {
  return `console-test-${workspace.context.tenantId || 'tenant'}-${workspace.context.projectId || 'project'}`
}

async function openIntegrationTest(app: EmbedAppRecord) {
  if (app.status === 'disabled') {
    notify.warning('请先启用内嵌应用再测试')
    return
  }
  integrationTestApp.value = app
  integrationTestForm.external_user_id = defaultTestUserID()
  integrationTestForm.external_user_name = '控制台测试用户'
  integrationTestForm.key = app.session_policy === 'user' ? 'default' : 'dashboard:123'
  integrationTestForm.session_mode = 'new'
  integrationTestForm.parent_origin = defaultParentOrigin(app)
  integrationTestVisible.value = true
  await runIntegrationTest()
}

async function runIntegrationTest() {
  const app = integrationTestApp.value
  if (!app) return false
  const externalUserID = integrationTestForm.external_user_id.trim()
  if (!externalUserID) {
    notify.warning('请输入测试用户 ID')
    return false
  }
  const result = await project.createEmbedIntegrationTest(app, {
    external_user_id: externalUserID,
    external_user_name: integrationTestForm.external_user_name.trim(),
    ttl_seconds: 3600
  })
  if (!result) return false
  integrationTestExpiresAt.value = result.expires_at
  integrationTestDoc.value = integrationTestHTML(app, result.access_token)
  return true
}

async function applyIntegrationTestConfig() {
  const ok = await runIntegrationTest()
  if (ok) integrationTestConfigVisible.value = false
}

function integrationTestHTML(app: EmbedAppRecord, token: string) {
  const sdkSrc = `${origin.value}/sdk/ling-shu-embed.js`
  const appTitle = escapeHTML(app.name || 'Ling-Shu')
  const parentOrigin = integrationTestForm.parent_origin.trim() || origin.value
  const sessionKey = integrationTestForm.key.trim() || 'default'
  const sessionMode = integrationTestForm.session_mode
  const userID = escapeHTML(integrationTestForm.external_user_id)
  const userName = escapeHTML(integrationTestForm.external_user_name || '测试用户')
  return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <style>
    *{box-sizing:border-box}
    html,body{width:100%;height:100%;margin:0}
    body{font-family:Aptos,"PingFang SC","Microsoft YaHei",sans-serif;background:#eef4f1;color:#17211f;overflow:hidden}
    .page{height:100%;min-height:100%;padding:22px;background:linear-gradient(135deg,rgba(15,143,107,.12),transparent 34%),linear-gradient(180deg,#fbfdfb,#eef4f1)}
    .shell{display:grid;grid-template-columns:220px minmax(0,1fr);gap:18px;height:100%}
    .side{border:1px solid #d6e4de;border-radius:12px;padding:16px;background:rgba(255,255,255,.86);box-shadow:0 18px 48px rgba(33,48,42,.08)}
    .mark{display:grid;width:44px;height:44px;place-items:center;border-radius:10px;background:#10241b;color:#fff;font-weight:900}
    .side h1{margin:12px 0 4px;font-size:19px;line-height:1.25}
    .side p{margin:0;color:#6a7872;font-size:12px;line-height:1.6}
    .nav{display:grid;gap:8px;margin-top:22px}
    .nav span{border-radius:8px;padding:9px 10px;color:#485951;font-size:13px;font-weight:800}
    .nav span:first-child{background:#e8f5ef;color:#0b704f}
    .main{min-width:0;overflow:hidden}
    .bar{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:14px;border:1px solid #d6e4de;border-radius:12px;padding:14px 16px;background:rgba(255,255,255,.88)}
    .brand{display:grid;gap:3px;min-width:0}
    .brand strong{font-size:20px}
    .brand span{overflow:hidden;color:#65736d;font-size:12px;text-overflow:ellipsis;white-space:nowrap}
    .user{display:grid;gap:2px;min-width:150px;text-align:right}
    .user strong{font-size:13px}
    .user span{color:#6b7a73;font-size:12px}
    .grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}
    .tile{min-height:112px;border:1px solid #dce7e1;border-radius:12px;padding:14px;background:#fff;box-shadow:0 12px 34px rgba(31,47,39,.06)}
    .tile span{display:block;color:#64736d;font-size:12px;font-weight:800}
    .tile strong{display:block;margin-top:12px;font-size:23px}
    .panel{display:grid;grid-template-columns:minmax(0,1.25fr) minmax(260px,.75fr);gap:12px;margin-top:12px}
    .chart,.list{min-height:250px;border:1px solid #dce7e1;border-radius:12px;padding:16px;background:#fff}
    .chart h2,.list h2{margin:0 0 14px;font-size:15px}
    .bars{display:flex;align-items:end;gap:12px;height:160px;border-bottom:1px solid #e2ebe6;padding:0 4px 12px}
    .barcol{flex:1;border-radius:8px 8px 3px 3px;background:linear-gradient(180deg,#28b98b,#0f7d5f)}
    .barcol:nth-child(2){height:68%}.barcol:nth-child(3){height:82%}.barcol:nth-child(4){height:46%}.barcol:nth-child(5){height:74%}.barcol:nth-child(6){height:58%}
    .barcol:nth-child(1){height:52%}
    .rows{display:grid;gap:10px}
    .row{display:flex;justify-content:space-between;gap:12px;border-bottom:1px solid #edf3ef;padding-bottom:9px;color:#506159;font-size:13px}
    .row strong{color:#17211f}
    @media(max-width:860px){.shell{grid-template-columns:1fr}.side{display:none}.panel{grid-template-columns:1fr}.grid{grid-template-columns:1fr}.page{padding:14px}}
  </style>
</head>
<body>
  <main class="page">
    <div class="shell">
      <aside class="side">
        <div class="mark">测</div>
        <h1>${appTitle}</h1>
        <p>这是一张模拟第三方系统页面，右下角机器人由正式 SDK 加载。</p>
        <nav class="nav">
          <span>经营看板</span>
          <span>客户分析</span>
          <span>订单明细</span>
          <span>配置检查</span>
        </nav>
      </aside>
      <div class="main">
        <div class="bar">
          <div class="brand">
            <strong>经营看板</strong>
            <span>模拟来源：${escapeHTML(parentOrigin)}</span>
          </div>
          <div class="user">
            <strong>${userName}</strong>
            <span>${userID}</span>
          </div>
        </div>
        <section class="grid">
          <div class="tile"><span>业务上下文</span><strong>${escapeHTML(sessionKey)}</strong></div>
          <div class="tile"><span>今日 GMV</span><strong>128.6w</strong></div>
          <div class="tile"><span>会话模式</span><strong>${sessionMode === 'new' ? '新会话' : '复用'}</strong></div>
        </section>
        <section class="panel">
          <div class="chart">
            <h2>近 6 小时交易趋势</h2>
            <div class="bars">
              <i class="barcol"></i><i class="barcol"></i><i class="barcol"></i><i class="barcol"></i><i class="barcol"></i><i class="barcol"></i>
            </div>
          </div>
          <div class="list">
            <h2>配置诊断</h2>
            <div class="rows">
              <div class="row"><span>SDK</span><strong>已加载</strong></div>
              <div class="row"><span>Token</span><strong>已签发</strong></div>
              <div class="row"><span>来源</span><strong>待校验</strong></div>
              <div class="row"><span>ASR/TTS</span><strong>自动探测</strong></div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </main>
  <script src="${escapeAttr(sdkSrc)}"><\/script>
  <script>
    window.LingShuEmbed.init({
      appId: ${jsonForScript(app.app_id)},
      baseUrl: ${jsonForScript(origin.value)},
      parentOrigin: ${jsonForScript(parentOrigin)},
      key: ${jsonForScript(sessionKey)},
      sessionMode: ${jsonForScript(sessionMode)},
      tokenProvider: function () {
        return Promise.resolve({ access_token: ${jsonForScript(token)} })
      },
      position: "bottom-right",
      autoOpen: true,
      launcher: { title: ${jsonForScript(app.launcher_title || '智能问数')} }
    })
  <\/script>
</body>
</html>`
}

function escapeHTML(value: string) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function escapeAttr(value: string) {
  return escapeHTML(value).replace(/'/g, '&#39;')
}

function jsonForScript(value: string) {
  return JSON.stringify(value).replace(/</g, '\\u003c')
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    notify.success('已复制')
  } catch {
    notify.warning('复制失败，请手动选择代码')
  }
}
</script>

<template>
  <NModal
    v-model:show="projectEmbedModalVisible"
    preset="card"
    class="project-embed-modal"
    :mask-closable="false"
  >
    <template #header>
      <div class="modal-title-stack">
        <span>内嵌集成</span>
        <small>{{ selectedProject?.name || `项目 #${embedProjectId}` }}</small>
      </div>
    </template>

    <div class="embed-modal-grid">
      <section class="embed-create-panel">
        <h3>创建机器人入口</h3>
        <NForm label-placement="top">
          <NFormItem label="应用名称">
            <NInput v-model:value="embedForm.name" placeholder="例如：经营看板助手" />
          </NFormItem>
          <NFormItem label="悬浮入口标题">
            <NInput v-model:value="embedForm.launcher_title" placeholder="智能问数" />
          </NFormItem>
          <NFormItem label="默认会话策略">
            <NSelect v-model:value="embedForm.session_policy" :options="sessionPolicyOptions" />
          </NFormItem>
          <NFormItem label="允许嵌入来源">
            <NInput
              v-model:value="embedForm.allowed_origins"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="https://console.example.com，一行一个或用逗号分隔"
            />
          </NFormItem>
          <NFormItem label="欢迎语">
            <NInput
              v-model:value="embedForm.welcome_message"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 3 }"
            />
          </NFormItem>
          <NButton type="primary" :loading="workspace.loading" @click="project.createEmbedApp">创建内嵌应用</NButton>
        </NForm>
      </section>

      <section class="embed-app-list">
        <div class="surface-head compact">
          <div>
            <h3>应用列表</h3>
            <p class="surface-note">三方页面加载 SDK 后会出现悬浮小机器人。</p>
          </div>
          <NButton size="small" secondary @click="project.refreshEmbedApps({ silent: true })">刷新</NButton>
        </div>

        <NAlert v-if="lastCreatedEmbed" type="success" title="App Secret 已生成" class="embed-secret-alert">
          <p>Secret 会加密保存在服务端，后续可在有项目管理权限时查看。</p>
          <code>{{ lastCreatedEmbed.app_secret }}</code>
          <NButton size="small" secondary @click="copyText(lastCreatedEmbed.app_secret)">复制 Secret</NButton>
        </NAlert>

        <div v-if="embedApps.items.length" class="embed-app-stack">
          <article v-for="app in embedApps.items" :key="app.id" class="embed-app-card">
            <header>
              <div>
                <strong>{{ app.name }}</strong>
                <p>{{ app.app_id }}</p>
              </div>
              <div class="embed-app-tags">
                <NTag size="small" round>{{ policyLabel(app.session_policy) }}</NTag>
                <NTag size="small" round :type="statusType(app.status)">{{ statusLabel(app.status) }}</NTag>
              </div>
            </header>
            <div v-if="displayedSecret(app)" class="embed-secret-inline">
              <code>{{ displayedSecret(app) }}</code>
              <NButton size="tiny" secondary @click="copyText(displayedSecret(app))">复制 Secret</NButton>
            </div>
            <NSpace size="small">
              <NButton size="small" secondary @click="copyText(app.app_id)">复制 appId</NButton>
              <NButton size="small" secondary @click="project.revealEmbedSecret(app)">查看 Secret</NButton>
              <NButton size="small" type="primary" secondary :disabled="app.status === 'disabled'" @click="openIntegrationTest(app)">
                集成测试
              </NButton>
              <NButton size="small" secondary @click="copyText(integrationCode(app))">复制 SDK 代码</NButton>
              <NButton
                v-if="app.status === 'disabled'"
                size="small"
                type="success"
                secondary
                @click="project.updateEmbedAppStatus(app, 'active')"
              >
                启用
              </NButton>
              <NButton
                v-else
                size="small"
                type="warning"
                secondary
                @click="project.updateEmbedAppStatus(app, 'disabled')"
              >
                停用
              </NButton>
              <NPopconfirm @positive-click="project.deleteEmbedApp(app)">
                <template #trigger>
                  <NButton size="small" type="error" quaternary>删除</NButton>
                </template>
                删除后这个 appId 将无法再签发 Token 或启动嵌入会话，确定删除吗？
              </NPopconfirm>
            </NSpace>
            <details class="embed-code-block">
              <summary>查看集成代码</summary>
              <pre>{{ integrationCode(app) }}</pre>
              <pre>{{ tokenServerExample(app, displayedSecret(app) || undefined) }}</pre>
            </details>
          </article>
        </div>
        <div v-else class="empty-state compact">
          <h2>还没有内嵌应用</h2>
          <p>创建后即可把智能问数机器人嵌入第三方系统。</p>
        </div>
      </section>
    </div>
  </NModal>

  <NModal
    v-model:show="integrationTestVisible"
    preset="card"
    class="embed-test-modal"
    :mask-closable="false"
  >
    <template #header>
      <div class="embed-test-header">
        <div class="modal-title-stack">
          <span>集成测试</span>
          <small>{{ integrationTestApp?.name || '内嵌应用' }}</small>
        </div>
        <div class="embed-test-toolbar">
          <div class="embed-test-status">
            <strong>{{ integrationTestForm.external_user_name || '测试用户' }}</strong>
            <span>
              {{ integrationTestForm.external_user_id || '-' }} · {{ integrationTestForm.key || 'default' }} ·
              {{ integrationTestForm.session_mode === 'new' ? '新会话' : '复用' }}
            </span>
            <small v-if="integrationTestExpiresAt">Token 到期：{{ integrationTestExpiresAt }}</small>
          </div>
          <NSpace size="small">
            <NButton secondary @click="integrationTestConfigVisible = true">
              <template #icon>
                <NIcon :component="Settings2" />
              </template>
              测试配置
            </NButton>
            <NButton type="primary" secondary :loading="workspace.loading" @click="runIntegrationTest">
              <template #icon>
                <NIcon :component="RotateCw" />
              </template>
              重新测试
            </NButton>
          </NSpace>
        </div>
      </div>
    </template>

    <div class="embed-test-layout">
      <section class="embed-test-preview">
        <iframe
          v-if="integrationTestDoc"
          :key="integrationTestDoc"
          title="Ling-Shu 内嵌集成测试"
          :srcdoc="integrationTestDoc"
          allow="microphone; autoplay"
        />
        <div v-else class="embed-test-empty">正在准备测试页面</div>
      </section>
    </div>
  </NModal>

  <NModal
    v-model:show="integrationTestConfigVisible"
    preset="card"
    class="embed-test-config-modal"
    :mask-closable="false"
  >
    <template #header>
      <div class="modal-title-stack">
        <span>测试配置</span>
        <small>模拟第三方系统传入的用户、业务上下文和来源</small>
      </div>
    </template>

    <NForm label-placement="top">
      <div class="embed-test-config-grid">
        <NFormItem label="测试用户 ID">
          <NInput v-model:value="integrationTestForm.external_user_id" />
        </NFormItem>
        <NFormItem label="测试用户名称">
          <NInput v-model:value="integrationTestForm.external_user_name" />
        </NFormItem>
        <NFormItem label="业务 Key">
          <NInput v-model:value="integrationTestForm.key" />
        </NFormItem>
        <NFormItem label="会话模式">
          <NSelect v-model:value="integrationTestForm.session_mode" :options="sessionModeOptions" />
        </NFormItem>
        <NFormItem label="模拟三方来源">
          <NInput v-model:value="integrationTestForm.parent_origin" />
        </NFormItem>
      </div>
      <div class="modal-actions">
        <NButton secondary @click="integrationTestConfigVisible = false">取消</NButton>
        <NButton type="primary" :loading="workspace.loading" @click="applyIntegrationTestConfig">保存并重新测试</NButton>
      </div>
    </NForm>
  </NModal>
</template>
