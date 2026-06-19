<script setup lang="ts">
import { NButton, NForm, NFormItem, NGi, NGrid, NInput, NModal, NPagination, NSelect, NSpace, NTag } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useMemberStore, projectRoleOptions, tenantRoleOptions } from '@/stores/member'
import { memberAccountName, memberDisplayName, memberStatus, pageSize, showPagination } from '@/utils/format'

const workspace = useWorkspaceStore()
const ui = useUiStore()
const projectStore = useProjectStore()
const memberStore = useMemberStore()

const { context, loading, pageState } = storeToRefs(workspace)
const { activeMemberSub } = storeToRefs(ui)
const { projectOptions, projectSelectable } = storeToRefs(projectStore)
const {
  users,
  tenantMembers,
  projectMembers,
  memberInviteModalVisible,
  projectMemberModalVisible,
  memberForm,
  userOptions
} = storeToRefs(memberStore)
</script>

<template>
  <section class="module-page members-page">
    <div class="config-banner">
      <div>
        <span>账号与项目权限</span>
        <h2>先邀请组织成员，再决定他能进入哪些项目</h2>
        <p>组织成员只是账号池；加入项目并分配角色后，成员才可以在对应项目里创建会话、维护知识或查看结果。</p>
      </div>
      <NButton type="primary" secondary @click="() => memberStore.refreshMembers()">刷新成员</NButton>
    </div>

    <div v-if="activeMemberSub === 'projectAccess'" class="workflow-strip">
      <div>
        <strong>1</strong>
        <span>创建组织成员账号</span>
      </div>
      <div>
        <strong>2</strong>
        <span>选择当前项目</span>
      </div>
      <div>
        <strong>3</strong>
        <span>分配项目角色</span>
      </div>
    </div>

    <div v-if="activeMemberSub !== 'directory'" class="member-action-grid single">
      <section v-if="activeMemberSub === 'invite'" class="surface member-action-surface">
        <div class="surface-head">
          <div>
            <h2>组织账号池</h2>
            <p class="surface-note">成员账号先属于组织，进入具体项目需要在“项目授权”里单独分配。</p>
          </div>
          <NSpace size="small">
            <NTag size="small">{{ tenantMembers.total }} 个账号</NTag>
            <NButton size="small" type="primary" @click="memberInviteModalVisible = true">创建成员账号</NButton>
          </NSpace>
        </div>
        <div class="member-card-list">
          <article v-for="member in tenantMembers.items" :key="String(member.id)" class="member-card">
            <div class="member-avatar">{{ memberDisplayName(member).slice(0, 1).toUpperCase() }}</div>
            <div>
              <strong>{{ memberDisplayName(member) }}</strong>
              <span>{{ memberAccountName(member) }}</span>
            </div>
            <NTag size="small" :type="memberStatus(member) === 'active' ? 'success' : 'default'">{{ memberStatus(member) }}</NTag>
          </article>
          <div v-if="!tenantMembers.items.length" class="knowledge-empty-mini">暂无组织成员</div>
        </div>
        <div v-if="showPagination(tenantMembers)" class="pager-row compact">
          <NPagination
            :page="pageState.tenantMembers"
            :page-size="pageSize(tenantMembers)"
            :item-count="tenantMembers.total"
            @update:page="(page) => workspace.changePage('tenantMembers', page, memberStore.refreshMembers)"
          />
        </div>
      </section>

      <section v-if="activeMemberSub === 'projectAccess'" class="surface member-action-surface member-join-surface">
        <div class="surface-head">
          <div>
            <h2>项目成员授权</h2>
            <p class="surface-note">选择项目后，可以把组织账号加入项目并分配角色。</p>
          </div>
          <NButton size="small" type="primary" @click="projectMemberModalVisible = true">添加项目成员</NButton>
        </div>
        <div class="member-project-toolbar">
          <div>
            <span>当前项目</span>
            <NSelect
              :value="context.projectId || null"
              :options="projectOptions"
              :disabled="!projectSelectable"
              filterable
              placeholder="选择要加入的项目"
              @update:value="workspace.handleProjectChange"
            />
          </div>
          <NButton secondary @click="() => memberStore.refreshMembers()">刷新项目成员</NButton>
        </div>
        <div class="member-join-stats">
          <div>
            <span>可选账号</span>
            <strong>{{ users.total }}</strong>
          </div>
          <div>
            <span>当前项目成员</span>
            <strong>{{ projectMembers.total }}</strong>
          </div>
        </div>
        <div class="member-card-list">
          <article v-for="member in projectMembers.items" :key="String(member.id)" class="member-card">
            <div class="member-avatar">{{ memberDisplayName(member).slice(0, 1).toUpperCase() }}</div>
            <div>
              <strong>{{ memberDisplayName(member) }}</strong>
              <span>{{ memberAccountName(member) }}</span>
            </div>
            <NTag size="small" :type="memberStatus(member) === 'active' ? 'success' : 'default'">{{ memberStatus(member) }}</NTag>
          </article>
          <div v-if="!projectMembers.items.length" class="knowledge-empty-mini">当前项目暂无成员</div>
        </div>
        <div v-if="showPagination(projectMembers)" class="pager-row compact">
          <NPagination
            :page="pageState.projectMembers"
            :page-size="pageSize(projectMembers)"
            :item-count="projectMembers.total"
            @update:page="(page) => workspace.changePage('projectMembers', page, memberStore.refreshMembers)"
          />
        </div>
      </section>
    </div>

    <section v-if="activeMemberSub === 'directory'" class="surface member-directory-surface">
      <div class="surface-head">
        <div>
          <h2>成员目录</h2>
          <p class="surface-note">左侧是组织账号池，右侧是当前项目的协作成员。</p>
        </div>
        <NSpace size="small">
          <NTag size="small">组织 {{ tenantMembers.total }} / 项目 {{ projectMembers.total }}</NTag>
          <NButton size="small" secondary @click="() => memberStore.refreshMembers()">刷新目录</NButton>
        </NSpace>
      </div>
      <div class="member-project-toolbar">
        <div>
          <span>当前项目</span>
          <NSelect
            :value="context.projectId || null"
            :options="projectOptions"
            :disabled="!projectSelectable"
            filterable
            placeholder="选择要查看成员的项目"
            @update:value="workspace.handleProjectChange"
          />
        </div>
      </div>
      <div class="member-summary-grid">
        <div>
          <strong>{{ tenantMembers.total }}</strong>
          <span>组织成员</span>
        </div>
        <div>
          <strong>{{ projectMembers.total }}</strong>
          <span>当前项目成员</span>
        </div>
        <div>
          <strong>{{ users.total }}</strong>
          <span>可选账号</span>
        </div>
      </div>
      <div class="member-table-grid">
        <div>
          <h2 class="sub-title">组织账号池</h2>
          <div class="member-card-list">
            <article v-for="member in tenantMembers.items" :key="String(member.id)" class="member-card">
              <div class="member-avatar">{{ memberDisplayName(member).slice(0, 1).toUpperCase() }}</div>
              <div>
                <strong>{{ memberDisplayName(member) }}</strong>
                <span>{{ memberAccountName(member) }}</span>
              </div>
              <NTag size="small" :type="memberStatus(member) === 'active' ? 'success' : 'default'">{{ memberStatus(member) }}</NTag>
            </article>
            <div v-if="!tenantMembers.items.length" class="knowledge-empty-mini">暂无组织成员</div>
          </div>
          <div v-if="showPagination(tenantMembers)" class="pager-row compact">
            <NPagination
              :page="pageState.tenantMembers"
              :page-size="pageSize(tenantMembers)"
              :item-count="tenantMembers.total"
              @update:page="(page) => workspace.changePage('tenantMembers', page, memberStore.refreshMembers)"
            />
          </div>
        </div>
        <div>
          <h2 class="sub-title">当前项目成员</h2>
          <div class="member-card-list">
            <article v-for="member in projectMembers.items" :key="String(member.id)" class="member-card">
              <div class="member-avatar">{{ memberDisplayName(member).slice(0, 1).toUpperCase() }}</div>
              <div>
                <strong>{{ memberDisplayName(member) }}</strong>
                <span>{{ memberAccountName(member) }}</span>
              </div>
              <NTag size="small" :type="memberStatus(member) === 'active' ? 'success' : 'default'">{{ memberStatus(member) }}</NTag>
            </article>
            <div v-if="!projectMembers.items.length" class="knowledge-empty-mini">当前项目暂无成员</div>
          </div>
          <div v-if="showPagination(projectMembers)" class="pager-row compact">
            <NPagination
              :page="pageState.projectMembers"
              :page-size="pageSize(projectMembers)"
              :item-count="projectMembers.total"
              @update:page="(page) => workspace.changePage('projectMembers', page, memberStore.refreshMembers)"
            />
          </div>
        </div>
      </div>
    </section>

    <NModal
      v-model:show="memberInviteModalVisible"
      preset="card"
      title="创建成员账号"
      class="member-modal"
      :mask-closable="false"
    >
      <NForm label-placement="top">
        <NGrid :cols="2" :x-gap="12" :y-gap="2" responsive="screen">
          <NGi>
            <NFormItem label="登录账号">
              <NInput v-model:value="memberForm.username" placeholder="例如：analyst" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="成员姓名">
              <NInput v-model:value="memberForm.display_name" placeholder="例如：数据分析师" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="邮箱（可选）">
              <NInput v-model:value="memberForm.email" placeholder="用于后续通知和找回" />
            </NFormItem>
          </NGi>
          <NGi>
            <NFormItem label="初始密码">
              <NInput v-model:value="memberForm.password" type="password" show-password-on="click" />
            </NFormItem>
          </NGi>
          <NGi :span="2">
            <NFormItem label="组织权限">
              <NSelect v-model:value="memberForm.tenantRoleCode" :options="tenantRoleOptions" />
            </NFormItem>
          </NGi>
        </NGrid>
        <div class="modal-actions">
          <NButton @click="memberInviteModalVisible = false">取消</NButton>
          <NButton type="primary" :loading="loading" @click="memberStore.createTenantAccount">创建成员账号</NButton>
        </div>
      </NForm>
    </NModal>

    <NModal
      v-model:show="projectMemberModalVisible"
      preset="card"
      title="添加项目成员"
      class="member-modal"
      :mask-closable="false"
    >
      <NForm label-placement="top">
        <NFormItem label="当前项目">
          <NSelect
            :value="context.projectId || null"
            :options="projectOptions"
            :disabled="!projectSelectable"
            filterable
            placeholder="选择要加入的项目"
            @update:value="workspace.handleProjectChange"
          />
        </NFormItem>
        <NFormItem label="选择组织成员">
          <NSelect v-model:value="memberForm.projectUserId" :options="userOptions" filterable placeholder="选择要加入项目的人" />
        </NFormItem>
        <NFormItem label="项目角色">
          <NSelect v-model:value="memberForm.projectRoleCode" :options="projectRoleOptions" />
        </NFormItem>
        <div class="modal-actions">
          <NButton @click="projectMemberModalVisible = false">取消</NButton>
          <NButton type="primary" :loading="loading" @click="memberStore.addProjectMember">加入当前项目</NButton>
        </div>
      </NForm>
    </NModal>
  </section>
</template>
