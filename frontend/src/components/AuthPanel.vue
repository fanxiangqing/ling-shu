<script setup lang="ts">
import { reactive, ref } from 'vue'
import { NButton, NForm, NFormItem, NInput, NTabPane, NTabs, useMessage } from 'naive-ui'
import BrandMark from '@/components/common/BrandMark.vue'
import { authApi } from '@/api/resources'
import type { LoginResult } from '@/types/domain'

const emit = defineEmits<{
  login: [result: LoginResult]
}>()

const message = useMessage()
const loading = ref(false)

const loginForm = reactive({
  username: 'admin',
  password: 'admin123'
})

const userForm = reactive({
  username: 'admin',
  password: 'admin123',
  display_name: '管理员',
  email: '',
  tenant_name: '默认组织'
})

async function login() {
  loading.value = true
  try {
    const result = await authApi.login(loginForm)
    message.success('登录成功')
    emit('login', result)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '登录失败')
  } finally {
    loading.value = false
  }
}

async function createUser() {
  loading.value = true
  try {
    await authApi.createUser(userForm)
    message.success('主账号已创建，正在进入控制台')
    loginForm.username = userForm.username
    loginForm.password = userForm.password
    const result = await authApi.login(loginForm)
    emit('login', result)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '创建主账号失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="auth-screen">
    <section class="auth-copy">
      <div class="seal auth-seal">
        <BrandMark />
      </div>
      <p class="eyebrow">Ling-Shu Console</p>
      <h1>企业自然语言问数工作台</h1>
      <p>
        登录后可以选择项目，直接用自然语言提问。管理员也可以配置数据连接、
        业务知识、AI 能力和成员权限。
      </p>
    </section>

    <section class="auth-panel">
      <NTabs type="segment" animated>
        <NTabPane name="login" tab="登录">
          <NForm label-placement="top" :model="loginForm">
            <NFormItem label="用户名">
              <NInput v-model:value="loginForm.username" placeholder="admin" />
            </NFormItem>
            <NFormItem label="密码">
              <NInput
                v-model:value="loginForm.password"
                type="password"
                show-password-on="click"
                placeholder="admin123"
                @keydown.enter="login"
              />
            </NFormItem>
            <NButton type="primary" block :loading="loading" @click="login">进入控制台</NButton>
          </NForm>
        </NTabPane>
        <NTabPane name="register" tab="创建主账号">
          <NForm label-placement="top" :model="userForm">
            <NFormItem label="组织名称">
              <NInput v-model:value="userForm.tenant_name" placeholder="例如：华东销售中心" />
            </NFormItem>
            <NFormItem label="用户名">
              <NInput v-model:value="userForm.username" />
            </NFormItem>
            <NFormItem label="显示名">
              <NInput v-model:value="userForm.display_name" />
            </NFormItem>
            <NFormItem label="邮箱">
              <NInput v-model:value="userForm.email" />
            </NFormItem>
            <NFormItem label="密码">
              <NInput v-model:value="userForm.password" type="password" show-password-on="click" />
            </NFormItem>
            <NButton secondary block :loading="loading" @click="createUser">创建并进入</NButton>
          </NForm>
        </NTabPane>
      </NTabs>
    </section>
  </main>
</template>
