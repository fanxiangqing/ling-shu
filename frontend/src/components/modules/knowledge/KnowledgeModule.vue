<script setup lang="ts">
import { NButton, NForm, NFormItem, NIcon, NInput, NInputNumber, NModal, NPagination, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { BookOpen } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useKnowledgeStore, knowledgeStatusOptions } from '@/stores/knowledge'
import { pageSize, showPagination, termAliases } from '@/utils/format'

const workspace = useWorkspaceStore()
const ui = useUiStore()
const projectStore = useProjectStore()
const knowledgeStore = useKnowledgeStore()

const { context, loading, pageState, workspaceReady } = storeToRefs(workspace)
const { activeKnowledgeSub } = storeToRefs(ui)
const { projectOptions, projectSelectable, projectDatasourceOptions } = storeToRefs(projectStore)
const {
  terms,
  metrics,
  fewShots,
  knowledgeStatusFilter,
  termModalVisible,
  metricModalVisible,
  fewShotModalVisible,
  termForm,
  metricForm,
  fewShotForm,
  ragForm,
  ragSearchResult,
  ragRebuildResult,
  ragLastQuestion,
  ragVectorHits,
  ragHitCount,
  ragContextCount,
  ragResultSections,
  visibleTerms,
  visibleMetrics,
  visibleFewShots,
  knowledgeTotal,
  enabledKnowledgeTotal
} = storeToRefs(knowledgeStore)

function ragHitTypeLabel(type: string) {
  const labels: Record<string, string> = {
    term: '术语',
    metric: '指标',
    fewshot: '示例'
  }
  return labels[type] || type || '知识'
}

function formatRAGScore(score: number) {
  if (!Number.isFinite(score)) return '0.000'
  return score.toFixed(3)
}

function ragHitTitle(hit: { kb_type: string; ref_id: number }) {
  const prefix = ragHitTypeLabel(hit.kb_type)
  return hit.ref_id ? `${prefix} #${hit.ref_id}` : prefix
}
</script>

<template>
  <section class="module-page knowledge-page">
    <div class="config-banner">
      <div>
        <span>增强问数理解</span>
        <h2>把业务语言教给当前项目</h2>
        <p>术语解释、指标口径和示例问法会在用户提问时进入 RAG，让系统知道“GMV”“销售额”“新增用户”在这个项目里到底怎么算。</p>
      </div>
      <NButton type="primary" secondary @click="() => knowledgeStore.refreshKnowledge()">刷新知识</NButton>
    </div>

    <div v-if="!workspaceReady" class="empty-state">
      <NIcon :component="BookOpen" />
      <h2>请先选择项目</h2>
      <p>业务知识必须归属到项目，因为不同项目的数据源和口径可能不同。</p>
      <NSelect
        :value="context.projectId || null"
        :options="projectOptions"
        :disabled="!projectSelectable"
        filterable
        placeholder="选择要维护知识的项目"
        @update:value="workspace.handleProjectChange"
      />
    </div>

    <template v-else>
      <div class="knowledge-control-bar">
        <div class="knowledge-project-control">
          <span>当前项目</span>
          <NSelect
            :value="context.projectId || null"
            :options="projectOptions"
            filterable
            placeholder="选择项目"
            @update:value="workspace.handleProjectChange"
          />
        </div>
        <div v-if="activeKnowledgeSub !== 'terms'" class="knowledge-project-control">
          <span>关联数据源</span>
          <NSelect
            :value="context.datasourceId || null"
            :options="projectDatasourceOptions"
            clearable
            filterable
            placeholder="全部数据源"
            @update:value="workspace.handleDatasourceChange"
          />
        </div>
        <div v-if="activeKnowledgeSub !== 'rag'" class="knowledge-project-control">
          <span>状态</span>
          <NSelect v-model:value="knowledgeStatusFilter" :options="knowledgeStatusOptions" />
        </div>
        <div v-if="activeKnowledgeSub !== 'rag'" class="knowledge-summary-tile">
          <span>已启用</span>
          <strong>{{ enabledKnowledgeTotal }} / {{ knowledgeTotal }}</strong>
        </div>
      </div>

      <div v-if="activeKnowledgeSub !== 'rag'" class="knowledge-grid single">
        <section v-if="activeKnowledgeSub === 'terms'" class="surface knowledge-card">
          <div class="surface-head">
            <div>
              <h2>业务术语</h2>
              <p class="surface-note">解释业务里的简称、黑话和同义词。</p>
            </div>
            <NSpace size="small">
              <NTag size="small">{{ terms.total }} 条</NTag>
              <NButton size="small" type="primary" @click="termModalVisible = true">新增术语</NButton>
            </NSpace>
          </div>
          <div class="knowledge-item-list">
            <article v-for="term in visibleTerms" :key="term.id" class="knowledge-item" :class="{ disabled: !term.enabled }">
              <div>
                <div class="knowledge-item-title">
                  <strong>{{ term.term }}</strong>
                  <NTag size="small" :type="term.enabled ? 'success' : 'default'">{{ term.enabled ? '启用' : '停用' }}</NTag>
                </div>
                <p>{{ term.definition }}</p>
                <span>{{ termAliases(term) }}</span>
              </div>
              <NSpace size="small">
                <NButton size="small" secondary @click="knowledgeStore.toggleTerm(term)">{{ term.enabled ? '停用' : '启用' }}</NButton>
                <NPopconfirm @positive-click="knowledgeStore.deleteTerm(term)">
                  <template #trigger>
                    <NButton size="small" quaternary type="error">删除</NButton>
                  </template>
                  删除后不会进入问数上下文，确认删除这个术语？
                </NPopconfirm>
              </NSpace>
            </article>
            <div v-if="!visibleTerms.length" class="knowledge-empty-mini">暂无术语</div>
          </div>
          <div v-if="showPagination(terms)" class="pager-row compact">
            <NPagination
              :page="pageState.terms"
              :page-size="pageSize(terms)"
              :item-count="terms.total"
              @update:page="(page) => workspace.changePage('terms', page, knowledgeStore.refreshKnowledge)"
            />
          </div>
        </section>

        <section v-if="activeKnowledgeSub === 'metrics'" class="surface knowledge-card">
          <div class="surface-head">
            <div>
              <h2>指标口径</h2>
              <p class="surface-note">固定指标公式、默认时间字段和关联数据源。</p>
            </div>
            <NSpace size="small">
              <NTag size="small">{{ metrics.total }} 条</NTag>
              <NButton size="small" type="primary" @click="metricModalVisible = true">新增指标</NButton>
            </NSpace>
          </div>
          <div class="knowledge-item-list">
            <article v-for="metric in visibleMetrics" :key="metric.id" class="knowledge-item" :class="{ disabled: !metric.enabled }">
              <div>
                <div class="knowledge-item-title">
                  <strong>{{ metric.name }}</strong>
                  <NTag size="small" :type="metric.enabled ? 'success' : 'default'">{{ metric.enabled ? '启用' : '停用' }}</NTag>
                </div>
                <p>{{ metric.description || '暂无业务说明' }}</p>
                <code>{{ metric.formula }}</code>
                <span>{{ metric.datasource_id ? `数据源 #${metric.datasource_id}` : '项目通用指标' }}</span>
              </div>
              <NSpace size="small">
                <NButton size="small" secondary @click="knowledgeStore.toggleMetric(metric)">{{ metric.enabled ? '停用' : '启用' }}</NButton>
                <NPopconfirm @positive-click="knowledgeStore.deleteMetric(metric)">
                  <template #trigger>
                    <NButton size="small" quaternary type="error">删除</NButton>
                  </template>
                  删除后不会进入问数上下文，确认删除这个指标？
                </NPopconfirm>
              </NSpace>
            </article>
            <div v-if="!visibleMetrics.length" class="knowledge-empty-mini">暂无指标</div>
          </div>
          <div v-if="showPagination(metrics)" class="pager-row compact">
            <NPagination
              :page="pageState.metrics"
              :page-size="pageSize(metrics)"
              :item-count="metrics.total"
              @update:page="(page) => workspace.changePage('metrics', page, knowledgeStore.refreshKnowledge)"
            />
          </div>
        </section>

        <section v-if="activeKnowledgeSub === 'fewShots'" class="surface knowledge-card">
          <div class="surface-head">
            <div>
              <h2>示例问法</h2>
              <p class="surface-note">给系统一个标准问法和 SQL 示例，复杂问题会更稳。</p>
            </div>
            <NSpace size="small">
              <NTag size="small">{{ fewShots.total }} 条</NTag>
              <NButton size="small" type="primary" @click="fewShotModalVisible = true">新增示例</NButton>
            </NSpace>
          </div>
          <div class="knowledge-item-list">
            <article v-for="fewShot in visibleFewShots" :key="fewShot.id" class="knowledge-item" :class="{ disabled: !fewShot.enabled }">
              <div>
                <div class="knowledge-item-title">
                  <strong>{{ fewShot.question }}</strong>
                  <NTag size="small" :type="fewShot.enabled ? 'success' : 'default'">{{ fewShot.enabled ? '启用' : '停用' }}</NTag>
                </div>
                <code>{{ fewShot.sql_text }}</code>
                <span>{{ fewShot.explanation || '暂无解释' }}</span>
              </div>
              <NSpace size="small">
                <NButton size="small" secondary @click="knowledgeStore.toggleFewShot(fewShot)">{{ fewShot.enabled ? '停用' : '启用' }}</NButton>
                <NPopconfirm @positive-click="knowledgeStore.deleteFewShot(fewShot)">
                  <template #trigger>
                    <NButton size="small" quaternary type="error">删除</NButton>
                  </template>
                  删除后不会进入问数上下文，确认删除这个示例？
                </NPopconfirm>
              </NSpace>
            </article>
            <div v-if="!visibleFewShots.length" class="knowledge-empty-mini">暂无示例</div>
          </div>
          <div v-if="showPagination(fewShots)" class="pager-row compact">
            <NPagination
              :page="pageState.fewShots"
              :page-size="pageSize(fewShots)"
              :item-count="fewShots.total"
              @update:page="(page) => workspace.changePage('fewShots', page, knowledgeStore.refreshKnowledge)"
            />
          </div>
        </section>
      </div>

      <section v-if="activeKnowledgeSub === 'rag'" class="surface knowledge-maintenance">
        <div class="surface-head">
          <div>
            <h2>知识索引维护</h2>
            <p class="surface-note">新增或修改知识后重建索引；不确定是否命中时，可以用一句业务问题检索看看。</p>
          </div>
          <NTag size="small">RAG</NTag>
        </div>
        <div class="knowledge-maintenance-row">
          <NInput v-model:value="ragForm.question" placeholder="输入一句业务问题，检查能否检索到相关知识" />
          <NInputNumber v-model:value="ragForm.limit" :min="1" :max="20" />
          <NButton :loading="loading" :disabled="!(ragForm.question || '').trim()" @click="knowledgeStore.searchRAG">检索知识</NButton>
          <NButton type="primary" secondary :loading="loading" @click="knowledgeStore.rebuildRAG">重建索引</NButton>
        </div>

        <div v-if="ragRebuildResult" class="rag-rebuild-strip">
          <div>
            <span>索引集合</span>
            <strong>{{ ragRebuildResult.collection || '默认集合' }}</strong>
          </div>
          <div>
            <span>知识切片</span>
            <strong>{{ ragRebuildResult.chunk_count || 0 }}</strong>
          </div>
          <div>
            <span>向量写入</span>
            <strong>{{ ragRebuildResult.vector_count || 0 }}</strong>
          </div>
          <div>
            <span>Embedding</span>
            <strong>{{ ragRebuildResult.embedding_model || '未返回' }}</strong>
          </div>
        </div>

        <div v-if="ragSearchResult" class="rag-result-area">
          <div class="rag-summary-strip">
            <div>
              <span>测试问题</span>
              <strong>{{ ragLastQuestion || ragForm.question }}</strong>
            </div>
            <div>
              <span>进入上下文</span>
              <strong>{{ ragContextCount }}</strong>
            </div>
            <div>
              <span>向量命中</span>
              <strong>{{ ragHitCount }}</strong>
            </div>
          </div>

          <div v-if="ragResultSections.length" class="rag-result-grid">
            <section v-for="section in ragResultSections" :key="section.key" class="rag-result-section">
              <div class="rag-result-section-head">
                <strong>{{ section.label }}</strong>
                <NTag size="small">{{ section.items.length }} 条</NTag>
              </div>
              <article v-for="item in section.items" :key="`${section.key}-${item.title}-${item.body}`" class="rag-result-item">
                <strong>{{ item.title }}</strong>
                <p>{{ item.body }}</p>
                <span v-if="item.meta">{{ item.meta }}</span>
                <code v-if="item.code">{{ item.code }}</code>
              </article>
            </section>
          </div>

          <section v-if="ragVectorHits.length" class="rag-hit-section">
            <div class="rag-result-section-head">
              <strong>向量命中片段</strong>
              <NTag size="small">{{ ragVectorHits.length }} 条</NTag>
            </div>
            <article v-for="hit in ragVectorHits" :key="hit.id" class="rag-hit-item">
              <div class="rag-hit-meta">
                <strong>{{ ragHitTitle(hit) }}</strong>
                <NTag size="small">相似度 {{ formatRAGScore(hit.score) }}</NTag>
                <NTag v-if="hit.datasource_id" size="small" type="info">数据源 #{{ hit.datasource_id }}</NTag>
              </div>
              <p>{{ hit.chunk_text }}</p>
            </article>
          </section>

          <div v-if="!ragResultSections.length && !ragVectorHits.length" class="knowledge-empty-mini rag-empty">
            本次没有可进入上下文的知识
          </div>
        </div>
      </section>
    </template>

    <NModal
      v-model:show="termModalVisible"
      preset="card"
      title="新增业务术语"
      class="knowledge-modal"
      :mask-closable="false"
    >
      <NForm label-placement="top">
        <NFormItem label="术语名称">
          <NInput v-model:value="termForm.term" placeholder="例如：GMV" />
        </NFormItem>
        <NFormItem label="别名">
          <NInput v-model:value="termForm.aliases" placeholder="例如：销售额,成交额" />
        </NFormItem>
        <NFormItem label="业务解释">
          <NInput v-model:value="termForm.definition" type="textarea" placeholder="说明这个词在当前项目里的含义" />
        </NFormItem>
        <div class="modal-actions">
          <NButton @click="termModalVisible = false">取消</NButton>
          <NButton type="primary" :loading="loading" @click="knowledgeStore.createTerm">保存术语</NButton>
        </div>
      </NForm>
    </NModal>

    <NModal
      v-model:show="metricModalVisible"
      preset="card"
      title="新增指标口径"
      class="knowledge-modal"
      :mask-closable="false"
    >
      <NForm label-placement="top">
        <NFormItem label="关联数据源">
          <NSelect
            :value="context.datasourceId || null"
            :options="projectDatasourceOptions"
            clearable
            filterable
            placeholder="可选；不选则作为项目通用指标"
            @update:value="workspace.handleDatasourceChange"
          />
        </NFormItem>
        <NFormItem label="指标名称">
          <NInput v-model:value="metricForm.name" placeholder="例如：销售额" />
        </NFormItem>
        <NFormItem label="业务说明">
          <NInput v-model:value="metricForm.description" placeholder="例如：已支付订单金额" />
        </NFormItem>
        <NFormItem label="计算口径">
          <NInput v-model:value="metricForm.formula" placeholder="例如：sum(pay_amount)" />
        </NFormItem>
        <NFormItem label="默认时间字段">
          <NInput v-model:value="metricForm.default_time_column" placeholder="例如：created_at" />
        </NFormItem>
        <div class="modal-actions">
          <NButton @click="metricModalVisible = false">取消</NButton>
          <NButton type="primary" :loading="loading" @click="knowledgeStore.createMetric">保存指标</NButton>
        </div>
      </NForm>
    </NModal>

    <NModal
      v-model:show="fewShotModalVisible"
      preset="card"
      title="新增示例问法"
      class="knowledge-modal"
      :mask-closable="false"
    >
      <NForm label-placement="top">
        <NFormItem label="用户会怎么问">
          <NInput v-model:value="fewShotForm.question" placeholder="例如：今天销售额是多少？" />
        </NFormItem>
        <NFormItem label="期望 SQL">
          <NInput v-model:value="fewShotForm.sql" type="textarea" placeholder="写一条安全的 SELECT 示例" />
        </NFormItem>
        <NFormItem label="解释">
          <NInput v-model:value="fewShotForm.explanation" placeholder="说明这个 SQL 的过滤和聚合口径" />
        </NFormItem>
        <div class="modal-actions">
          <NButton @click="fewShotModalVisible = false">取消</NButton>
          <NButton type="primary" :loading="loading" @click="knowledgeStore.createFewShot">保存示例</NButton>
        </div>
      </NForm>
    </NModal>
  </section>
</template>
