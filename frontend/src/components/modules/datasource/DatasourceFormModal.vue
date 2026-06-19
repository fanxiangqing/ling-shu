<script setup lang="ts">
import { NButton, NForm, NFormItem, NGi, NGrid, NInput, NInputNumber, NModal, NSelect } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useDatasourceStore, dbTypeOptions } from '@/stores/datasource'

const workspace = useWorkspaceStore()
const datasourceStore = useDatasourceStore()

const { loading } = storeToRefs(workspace)
const { datasourceModalVisible, datasourceForm } = storeToRefs(datasourceStore)
</script>

<template>
  <NModal
    v-model:show="datasourceModalVisible"
    preset="card"
    title="添加数据源"
    class="datasource-modal"
    :mask-closable="false"
  >
    <NForm label-placement="top">
      <NGrid :cols="2" :x-gap="12" :y-gap="2" responsive="screen">
        <NGi>
          <NFormItem label="数据源名称">
            <NInput v-model:value="datasourceForm.name" placeholder="例如：销售库" />
          </NFormItem>
        </NGi>
        <NGi>
          <NFormItem label="数据源类型">
            <NSelect
              :value="datasourceForm.db_type"
              :options="dbTypeOptions"
              placeholder="选择数据库类型"
              @update:value="datasourceStore.handleDatasourceTypeChange"
            />
          </NFormItem>
        </NGi>
        <NGi>
          <NFormItem label="Host">
            <NInput v-model:value="datasourceForm.host" placeholder="127.0.0.1" />
          </NFormItem>
        </NGi>
        <NGi>
          <NFormItem label="Port">
            <NInputNumber v-model:value="datasourceForm.port" :min="1" :max="65535" class="full-input" />
          </NFormItem>
        </NGi>
        <NGi>
          <NFormItem label="Username">
            <NInput v-model:value="datasourceForm.username" placeholder="数据库用户名" />
          </NFormItem>
        </NGi>
        <NGi>
          <NFormItem label="Password">
            <NInput
              v-model:value="datasourceForm.password"
              type="password"
              show-password-on="click"
              placeholder="数据库密码"
            />
          </NFormItem>
        </NGi>
        <NGi :span="2">
          <NFormItem label="数据库">
            <NInput v-model:value="datasourceForm.database" placeholder="例如：ling_shu" />
          </NFormItem>
        </NGi>
        <NGi :span="2">
          <NFormItem label="高级配置">
            <NInput v-model:value="datasourceForm.config_json" type="textarea" placeholder="可选 JSON 配置" />
          </NFormItem>
        </NGi>
      </NGrid>
      <div class="modal-actions">
        <NButton @click="datasourceModalVisible = false">取消</NButton>
        <NButton secondary :loading="loading" @click="datasourceStore.testDatasourceForm">测试连接</NButton>
        <NButton type="primary" :loading="loading" @click="datasourceStore.createDatasource">保存数据源</NButton>
      </div>
    </NForm>
  </NModal>
</template>
