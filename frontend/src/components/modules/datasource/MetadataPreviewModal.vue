<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { NButton, NInput, NModal, NPagination, NSpace, NTag } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useDatasourceStore } from '@/stores/datasource'
import { foreignKeyTarget, indexColumns, pageSize, showPagination } from '@/utils/format'
import type { MetadataTableRecord } from '@/types/domain'

const workspace = useWorkspaceStore()
const datasourceStore = useDatasourceStore()

const { pageState } = storeToRefs(workspace)
const {
  metadataPreviewVisible,
  selectedDatasource,
  metadataTables,
  selectedMetadataTable,
  tableCommentDraft,
  columnCommentDrafts
} = storeToRefs(datasourceStore)

const detailPaneRef = ref<HTMLElement | null>(null)
const detailInnerScrollSelector = '.metadata-related-grid, .metadata-relation-list, .metadata-column-list'

function scrollDetailTargetsToTop() {
  const pane = detailPaneRef.value
  if (!pane) return

  const targets = new Set<HTMLElement>([pane])
  pane.querySelectorAll<HTMLElement>(detailInnerScrollSelector).forEach((target) => targets.add(target))
  const modalScrollContainer = pane.closest<HTMLElement>('.n-scrollbar-container')
  if (modalScrollContainer) targets.add(modalScrollContainer)

  targets.forEach((target) => {
    target.scrollTop = 0
    target.scrollLeft = 0
  })
}

function resetDetailScroll() {
  nextTick(() => {
    scrollDetailTargetsToTop()
    window.requestAnimationFrame(scrollDetailTargetsToTop)
  })
}

async function handleTableSelect(table: MetadataTableRecord) {
  scrollDetailTargetsToTop()
  await datasourceStore.selectMetadataTable(table)
  resetDetailScroll()
}

watch(() => selectedMetadataTable.value?.id, resetDetailScroll, { flush: 'post' })
</script>

<template>
  <NModal
    :show="metadataPreviewVisible"
    preset="card"
    :title="`${selectedDatasource?.name || '数据源'} 元数据`"
    class="metadata-modal"
    :mask-closable="false"
    @update:show="datasourceStore.handleMetadataModalVisibleChange"
  >
    <div class="metadata-modal-body">
      <div class="metadata-modal-toolbar">
        <p>查看表、字段、索引和外键，也可以维护给业务用户看的表和字段备注。</p>
        <NSpace size="small">
          <NTag size="small">{{ metadataTables.total }} 张表</NTag>
          <NButton size="small" secondary @click="datasourceStore.syncDatasource">同步元数据</NButton>
          <NButton size="small" @click="datasourceStore.resetMetadataPreview">关闭</NButton>
        </NSpace>
      </div>

      <div class="metadata-browser">
        <div class="metadata-table-pane">
          <div class="metadata-table-list">
            <button
              v-for="table in metadataTables.items"
              :key="table.id"
              type="button"
              class="metadata-table-row"
              :class="{ active: selectedMetadataTable?.id === table.id }"
              @click="handleTableSelect(table)"
            >
              <span>
                <strong>{{ table.table_name }}</strong>
                <em>{{ table.schema_name }} · {{ table.table_type || 'table' }}</em>
              </span>
              <small>{{ table.comment_text || '暂无备注' }}</small>
            </button>
            <div v-if="!metadataTables.items.length" class="metadata-empty">暂无表，请先同步元数据。</div>
          </div>
          <div v-if="showPagination(metadataTables)" class="pager-row compact">
            <NPagination
              :page="pageState.metadataTables"
              :page-size="pageSize(metadataTables)"
              :item-count="metadataTables.total"
              @update:page="(page) => workspace.changePage('metadataTables', page, datasourceStore.loadMetadata)"
            />
          </div>
        </div>

        <div ref="detailPaneRef" class="metadata-detail-card">
          <template v-if="selectedMetadataTable">
            <div class="metadata-detail-head">
              <div>
                <span>{{ selectedMetadataTable.schema_name }}</span>
                <h3>{{ selectedMetadataTable.table_name }}</h3>
              </div>
              <NSpace size="small">
                <NTag size="small">{{ selectedMetadataTable.columns?.length || 0 }} 个字段</NTag>
                <NTag size="small">{{ selectedMetadataTable.indexes?.length || 0 }} 个索引</NTag>
                <NTag size="small">{{ selectedMetadataTable.foreign_keys?.length || 0 }} 个外键</NTag>
              </NSpace>
            </div>

            <div class="metadata-comment-editor">
              <label>表备注</label>
              <div class="metadata-comment-row">
                <NInput v-model:value="tableCommentDraft" clearable placeholder="给业务用户看的表含义，例如：订单主表" />
                <NButton type="primary" secondary @click="datasourceStore.saveMetadataTableComment">保存</NButton>
              </div>
            </div>

            <div class="metadata-related-grid">
              <section>
                <div class="metadata-relation-head">
                  <h4>索引</h4>
                  <NTag size="small">{{ selectedMetadataTable.indexes?.length || 0 }} 个</NTag>
                </div>
                <div v-if="(selectedMetadataTable.indexes || []).length" class="metadata-relation-list">
                  <article
                    v-for="index in selectedMetadataTable.indexes || []"
                    :key="index.id"
                    class="metadata-relation-card"
                    :class="{ important: index.unique_index }"
                  >
                    <div>
                      <strong>{{ index.index_name }}</strong>
                      <NTag size="small" :type="index.unique_index ? 'success' : 'default'">
                        {{ index.unique_index ? '唯一索引' : index.index_type || '普通索引' }}
                      </NTag>
                    </div>
                    <span class="metadata-relation-meta">字段：{{ indexColumns(index) }}</span>
                  </article>
                </div>
                <div v-else class="metadata-empty compact">暂无索引。</div>
              </section>
              <section>
                <div class="metadata-relation-head">
                  <h4>外键</h4>
                  <NTag size="small">{{ selectedMetadataTable.foreign_keys?.length || 0 }} 个</NTag>
                </div>
                <div v-if="(selectedMetadataTable.foreign_keys || []).length" class="metadata-relation-list">
                  <article
                    v-for="fk in selectedMetadataTable.foreign_keys || []"
                    :key="fk.id"
                    class="metadata-relation-card foreign"
                  >
                    <div>
                      <strong>{{ fk.constraint_name || fk.column_name }}</strong>
                      <NTag size="small" type="warning">外键</NTag>
                    </div>
                    <span class="metadata-relation-meta">{{ fk.column_name }} → {{ foreignKeyTarget(fk) }}</span>
                  </article>
                </div>
                <div v-else class="metadata-empty compact">暂无外键。</div>
              </section>
            </div>

            <div class="metadata-section metadata-fields-section">
              <div class="metadata-section-title">
                <h4>字段</h4>
                <span>字段备注会进入问数上下文，建议写业务含义而不是技术说明。</span>
              </div>
              <div class="metadata-column-list">
                <div
                  v-for="column in selectedMetadataTable.columns || []"
                  :key="column.id"
                  class="metadata-column-row"
                >
                  <div class="metadata-column-main">
                    <strong>{{ column.column_name }}</strong>
                    <span>{{ column.column_type || column.data_type }}</span>
                    <div class="metadata-column-tags">
                      <NTag v-if="column.is_primary_key" size="small" type="success">主键</NTag>
                      <NTag v-if="column.is_foreign_key" size="small" type="warning">外键</NTag>
                      <NTag size="small">{{ column.nullable ? '可为空' : '必填' }}</NTag>
                    </div>
                  </div>
                  <div class="metadata-comment-row">
                    <NInput
                      v-model:value="columnCommentDrafts[column.id]"
                      clearable
                      placeholder="字段业务备注"
                    />
                    <NButton size="small" secondary @click="datasourceStore.saveMetadataColumnComment(column)">保存</NButton>
                  </div>
                </div>
                <div v-if="!(selectedMetadataTable.columns || []).length" class="metadata-empty">暂无字段。</div>
              </div>
            </div>
          </template>
          <div v-else class="metadata-empty detail">请选择左侧表查看字段详情。</div>
        </div>
      </div>
    </div>
  </NModal>
</template>
