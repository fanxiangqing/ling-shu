<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { NConfigProvider, NGlobalStyle, NMessageProvider, type GlobalThemeOverrides } from 'naive-ui'
import AuthPanel from '@/components/AuthPanel.vue'
import ManagementConsole from '@/components/ManagementConsole.vue'
import EmbedChatApp from '@/components/embed/EmbedChatApp.vue'
import { LOGIN_KEY, UNAUTHORIZED_EVENT, clearAuthState } from '@/api/client'
import type { LoginResult } from '@/types/domain'

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#0f8f6b',
    primaryColorHover: '#0a7557',
    primaryColorPressed: '#095f48',
    borderRadius: '8px',
    fontFamily: '"Aptos", "IBM Plex Sans", "PingFang SC", "Microsoft YaHei", sans-serif'
  },
  Button: {
    fontSizeMedium: '14px',
    fontSizeSmall: '12px',
    fontWeight: '700',
    fontWeightStrong: '800',
    heightMedium: '36px',
    heightSmall: '28px',
    paddingMedium: '0 14px',
    paddingSmall: '0 10px',
    borderRadiusMedium: '8px',
    borderRadiusSmall: '7px'
  },
  Input: {
    borderRadius: '8px'
  },
  DataTable: {
    thColor: '#edf2ec',
    tdColor: '#fbfaf6',
    borderColor: '#d8dfd8'
  }
}

const login = ref<LoginResult | null>(readLogin())
const isEmbedMode = window.location.pathname.startsWith('/embed/')

function readLogin() {
  const raw = localStorage.getItem(LOGIN_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as LoginResult
  } catch {
    return null
  }
}

function onLogin(result: LoginResult) {
  login.value = result
  localStorage.setItem(LOGIN_KEY, JSON.stringify(result))
}

function onLogout() {
  clearAuthState()
  login.value = null
}

function onUnauthorized() {
  onLogout()
}

onMounted(() => {
  window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
})

onBeforeUnmount(() => {
  window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
})
</script>

<template>
  <NConfigProvider :theme-overrides="themeOverrides">
    <NMessageProvider>
      <NGlobalStyle />
      <EmbedChatApp v-if="isEmbedMode" />
      <AuthPanel v-else-if="!login" @login="onLogin" />
      <ManagementConsole v-else :login="login" @logout="onLogout" />
    </NMessageProvider>
  </NConfigProvider>
</template>
