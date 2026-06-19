<script setup lang="ts">
import { NButton, NForm, NFormItem, NGi, NGrid, NInput, NModal, NSelect, NTag } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useProjectStore } from '@/stores/project'
import { useDatasourceStore } from '@/stores/datasource'

const workspace = useWorkspaceStore()
const projectStore = useProjectStore()
const datasourceStore = useDatasourceStore()

const { loading } = storeToRefs(workspace)
const { projectModalVisible, projectForm } = storeToRefs(projectStore)
const { datasourceOptions } = storeToRefs(datasourceStore)

const llmConfigModeOptions = [
  { label: '使用 Global 配置', value: 'global' },
  { label: '项目自定义配置', value: 'custom' }
]

const optionalProviderModeOptions = [
  { label: '使用 Global 配置', value: 'global' },
  { label: '项目自定义配置', value: 'custom' },
  { label: '项目不启用', value: 'disabled' }
]
</script>

<template>
  <NModal
    v-model:show="projectModalVisible"
    preset="card"
    title="创建项目"
    class="project-modal"
    :mask-closable="false"
  >
    <NForm label-placement="top">
      <NGrid :cols="2" :x-gap="12" :y-gap="2" responsive="screen">
        <NGi :span="2">
          <NFormItem label="项目名称">
            <NInput v-model:value="projectForm.name" placeholder="例如：电商经营项目" />
          </NFormItem>
        </NGi>
        <NGi :span="2">
          <NFormItem label="项目说明">
            <NInput v-model:value="projectForm.description" placeholder="订单、商品、用户与渠道" />
          </NFormItem>
        </NGi>
        <NGi :span="2">
          <NFormItem label="绑定数据源">
            <NSelect
              v-model:value="projectForm.datasource_ids"
              :options="datasourceOptions"
              multiple
              filterable
              placeholder="选择这个项目可使用的数据源"
            />
          </NFormItem>
        </NGi>
      </NGrid>

      <div class="provider-config-grid">
        <section class="provider-config-card">
          <div class="provider-config-head">
            <h3>LLM</h3>
            <NTag size="small">默认 Global</NTag>
          </div>
          <NFormItem label="配置来源">
            <NSelect v-model:value="projectForm.llm_mode" :options="llmConfigModeOptions" />
          </NFormItem>
          <template v-if="projectForm.llm_mode === 'custom'">
            <NFormItem label="模型">
              <NInput v-model:value="projectForm.llm_model" />
            </NFormItem>
            <NFormItem label="API Base">
              <NInput v-model:value="projectForm.llm_api_base" />
            </NFormItem>
            <NFormItem label="API Key">
              <NInput v-model:value="projectForm.llm_api_key" type="password" show-password-on="click" />
            </NFormItem>
          </template>
        </section>

        <section class="provider-config-card">
          <div class="provider-config-head">
            <h3>ASR</h3>
            <NTag size="small">可选</NTag>
          </div>
          <NFormItem label="配置来源">
            <NSelect v-model:value="projectForm.asr_mode" :options="optionalProviderModeOptions" />
          </NFormItem>
          <template v-if="projectForm.asr_mode === 'custom'">
            <NFormItem label="模型">
              <NInput v-model:value="projectForm.asr_model" />
            </NFormItem>
            <NFormItem label="AccessKey ID">
              <NInput v-model:value="projectForm.asr_access_key_id" />
            </NFormItem>
            <NFormItem label="AccessKey Secret">
              <NInput v-model:value="projectForm.asr_access_key_secret" type="password" show-password-on="click" />
            </NFormItem>
            <NFormItem label="AppKey">
              <NInput v-model:value="projectForm.asr_app_key" />
            </NFormItem>
          </template>
        </section>

        <section class="provider-config-card">
          <div class="provider-config-head">
            <h3>TTS</h3>
            <NTag size="small">可选</NTag>
          </div>
          <NFormItem label="配置来源">
            <NSelect v-model:value="projectForm.tts_mode" :options="optionalProviderModeOptions" />
          </NFormItem>
          <template v-if="projectForm.tts_mode === 'custom'">
            <NFormItem label="模型">
              <NInput v-model:value="projectForm.tts_model" />
            </NFormItem>
            <NFormItem label="音色">
              <NInput v-model:value="projectForm.tts_voice" />
            </NFormItem>
            <NFormItem label="AccessKey ID">
              <NInput v-model:value="projectForm.tts_access_key_id" />
            </NFormItem>
            <NFormItem label="AccessKey Secret">
              <NInput v-model:value="projectForm.tts_access_key_secret" type="password" show-password-on="click" />
            </NFormItem>
            <NFormItem label="AppKey">
              <NInput v-model:value="projectForm.tts_app_key" />
            </NFormItem>
          </template>
        </section>
      </div>

      <div class="modal-actions">
        <NButton @click="projectModalVisible = false">取消</NButton>
        <NButton type="primary" :loading="loading" @click="projectStore.createProject">保存项目</NButton>
      </div>
    </NForm>
  </NModal>
</template>
