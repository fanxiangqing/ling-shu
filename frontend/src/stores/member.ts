import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { authApi, permissionApi } from '@/api/resources'
import type { MemberRecord, PageResult, UserRecord } from '@/types/domain'
import { emptyPage, memberDisplayName, memberStatus } from '@/utils/format'
import { fetchAllPages } from '@/utils/pagination'
import { useWorkspaceStore } from '@/stores/workspace'

export const tenantRoleOptions = [
  { label: '普通成员', value: '' },
  { label: '组织管理员', value: 'tenant_admin' }
]

export const projectRoleOptions = [
  { label: '项目管理员', value: 'project_admin' },
  { label: '分析师', value: 'analyst' },
  { label: '只读成员', value: 'viewer' }
]

export const useMemberStore = defineStore('member', () => {
  const ws = useWorkspaceStore()

  const users = ref<PageResult<UserRecord>>(emptyPage())
  const userOptionItems = ref<UserRecord[]>([])
  const tenantMemberOptionItems = ref<MemberRecord[]>([])
  const tenantMemberOptionTenantId = ref(0)
  const tenantMembers = ref<PageResult<MemberRecord>>(emptyPage())
  const projectMembers = ref<PageResult<MemberRecord>>(emptyPage())

  const memberInviteModalVisible = ref(false)
  const projectMemberModalVisible = ref(false)

  const memberForm = reactive({
    username: 'analyst',
    password: '123456',
    display_name: '数据分析师',
    email: '',
    tenantRoleCode: '',
    projectUserId: ws.context.userId,
    projectRoleCode: 'analyst'
  })

  const userOptionRecords = computed(() => userOptionItems.value.length ? userOptionItems.value : users.value.items)
  const tenantMemberOptionRecords = computed(() =>
    tenantMemberOptionItems.value.length ? tenantMemberOptionItems.value : tenantMembers.value.items
  )
  const userOptions = computed(() => {
    if (ws.context.tenantId) {
      return tenantMemberOptionRecords.value.filter((member) => memberStatus(member) === 'active').map((member) => ({
        label: `${String(member.display_name || member.username || '成员')} / ${String(member.username || member.user_id || '')}`,
        value: Number(member.user_id || member.id || 0)
      })).filter((option) => option.value > 0)
    }
    return userOptionRecords.value.map((user) => ({
      label: `${user.display_name || user.username} / ${user.username}`,
      value: user.id
    }))
  })

  async function refreshUsers(options: { silent?: boolean } = {}) {
    const result = await ws.run('刷新用户', () => authApi.listUsers(ws.pageParams('users')), options)
    if (result) {
      users.value = result as PageResult<UserRecord>
      ws.syncPage('users', users.value)
      await refreshUserOptions()
    }
  }

  async function refreshUserOptions() {
    const result = await ws.run(
      '刷新用户选项',
      () => fetchAllPages<UserRecord>((params) => authApi.listUsers(params)),
      { silent: true, successMessage: false }
    )
    if (result) userOptionItems.value = result as UserRecord[]
  }

  async function refreshMembers(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId) {
      tenantMembers.value = emptyPage()
      tenantMemberOptionItems.value = []
      tenantMemberOptionTenantId.value = 0
      projectMembers.value = emptyPage()
      return
    }
    const tenantResult = await ws.run('刷新组织成员', () => authApi.listTenantMembers(ws.context.tenantId, ws.pageParams('tenantMembers')), options)
    if (tenantResult) {
      tenantMembers.value = tenantResult as PageResult<MemberRecord>
      ws.syncPage('tenantMembers', tenantMembers.value)
      await refreshTenantMemberOptions()
    }
    if (!ws.context.projectId) {
      projectMembers.value = emptyPage()
      return
    }
    const projectResult = await ws.run('刷新项目成员', () => authApi.listProjectMembers(ws.context.projectId, ws.context.tenantId, ws.pageParams('projectMembers')), options)
    if (projectResult) {
      projectMembers.value = projectResult as PageResult<MemberRecord>
      ws.syncPage('projectMembers', projectMembers.value)
    }
  }

  async function refreshTenantMemberOptions() {
    if (!ws.context.tenantId) {
      tenantMemberOptionItems.value = []
      tenantMemberOptionTenantId.value = 0
      return
    }
    if (tenantMemberOptionTenantId.value !== ws.context.tenantId) {
      tenantMemberOptionItems.value = []
      tenantMemberOptionTenantId.value = ws.context.tenantId
    }
    const result = await ws.run(
      '刷新组织成员选项',
      () => fetchAllPages<MemberRecord>((params) => authApi.listTenantMembers(ws.context.tenantId, params)),
      { silent: true, successMessage: false }
    )
    if (result) tenantMemberOptionItems.value = result as MemberRecord[]
  }

  async function createTenantAccount() {
    if (!ws.ensureTenant()) return
    const result = await ws.run('创建成员账号', () =>
      authApi.createTenantUser(ws.context.tenantId, {
        username: memberForm.username,
        password: memberForm.password,
        display_name: memberForm.display_name,
        email: memberForm.email,
        role_code: memberForm.tenantRoleCode || undefined
      })
    )
    if (result) {
      memberInviteModalVisible.value = false
      await refreshUsers()
      await refreshMembers({ silent: true })
    }
  }

  async function addProjectMember() {
    if (!ws.ensureProject()) return
    const result = await ws.run('加入项目', async () => {
      const member = await authApi.addProjectMember(ws.context.projectId, { tenant_id: ws.context.tenantId, user_id: memberForm.projectUserId })
      if (memberForm.projectRoleCode) {
        await permissionApi.bindRole({
          user_id: memberForm.projectUserId,
          role_code: memberForm.projectRoleCode,
          tenant_id: ws.context.tenantId,
          project_id: ws.context.projectId,
          created_by: ws.context.userId
        })
      }
      return member
    })
    if (!result) return
    projectMemberModalVisible.value = false
    await refreshMembers()
  }

  async function updateTenantMemberStatus(member: MemberRecord, status: 'active' | 'inactive') {
    if (!ws.ensureTenant()) return
    const id = Number(member.id || 0)
    if (!id) return
    const result = await ws.run(
      status === 'active' ? '启用组织成员' : '停用组织成员',
      () => authApi.updateTenantMemberStatus(ws.context.tenantId, id, { status }),
      { successMessage: `${memberDisplayName(member)} 已${status === 'active' ? '启用' : '停用'}` }
    )
    if (result) await refreshMembers({ silent: true })
  }

  async function toggleTenantMemberStatus(member: MemberRecord) {
    await updateTenantMemberStatus(member, memberStatus(member) === 'active' ? 'inactive' : 'active')
  }

  async function deleteTenantMember(member: MemberRecord) {
    if (!ws.ensureTenant()) return
    const id = Number(member.id || 0)
    if (!id) return
    const result = await ws.run(
      '删除组织成员',
      () => authApi.deleteTenantMember(ws.context.tenantId, id),
      { successMessage: `${memberDisplayName(member)} 已从组织移除` }
    )
    if (result) await refreshMembers({ silent: true })
  }

  async function updateProjectMemberStatus(member: MemberRecord, status: 'active' | 'inactive') {
    if (!ws.ensureProject()) return
    const id = Number(member.id || 0)
    if (!id) return
    const result = await ws.run(
      status === 'active' ? '启用项目成员' : '停用项目成员',
      () => authApi.updateProjectMemberStatus(ws.context.projectId, ws.context.tenantId, id, { status }),
      { successMessage: `${memberDisplayName(member)} 已${status === 'active' ? '启用' : '停用'}` }
    )
    if (result) await refreshMembers({ silent: true })
  }

  async function toggleProjectMemberStatus(member: MemberRecord) {
    await updateProjectMemberStatus(member, memberStatus(member) === 'active' ? 'inactive' : 'active')
  }

  async function deleteProjectMember(member: MemberRecord) {
    if (!ws.ensureProject()) return
    const id = Number(member.id || 0)
    if (!id) return
    const result = await ws.run(
      '移除项目成员',
      () => authApi.deleteProjectMember(ws.context.projectId, ws.context.tenantId, id),
      { successMessage: `${memberDisplayName(member)} 已移出当前项目` }
    )
    if (result) await refreshMembers({ silent: true })
  }

  return {
    users,
    userOptionItems,
    tenantMemberOptionItems,
    tenantMembers,
    projectMembers,
    memberInviteModalVisible,
    projectMemberModalVisible,
    memberForm,
    userOptions,
    refreshUsers,
    refreshUserOptions,
    refreshTenantMemberOptions,
    refreshMembers,
    createTenantAccount,
    addProjectMember,
    updateTenantMemberStatus,
    toggleTenantMemberStatus,
    deleteTenantMember,
    updateProjectMemberStatus,
    toggleProjectMemberStatus,
    deleteProjectMember
  }
})
