import { createFileRoute } from '@tanstack/react-router'
import { AlertTriangle } from 'lucide-react'
import { EditorDesk } from '../features/workbench/editor-desk'
import { AttentionList } from '../features/workbench/dashboard/overview'
import { readOpsAttention } from '../features/workbench/feed'
import { Panel } from '../features/workbench/shared/ui'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/exceptions')({
  loader: ({ context }) => context.hasWorkbenchSession ? readOpsAttention() : null,
  component: ExceptionsPage,
})

function ExceptionsPage() {
  const data = Route.useLoaderData()
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  const selectedTask = search.task
  return <WorkbenchPageShell current="exceptions" title="异常与决策" english="EXCEPTIONS" description="集中处理离线进程、阻塞项、人工决策和编辑审稿任务。"
    updatedAt={data?.as_of ?? null} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    {data ? <AttentionList items={data.attention} /> : <Panel><h2>关注列表暂不可用</h2><p>无法读取运营关注数据。</p></Panel>}
    {selectedTask && <Panel><div className="wb-eyebrow">REVIEW TASK</div><p>已打开任务 {selectedTask} 的审稿详情。</p></Panel>}
    <Panel className="wb-editor-review">
      <div className="wb-section-heading"><div><span className="wb-eyebrow">EDITORIAL REVIEW</span><h2>编辑审稿</h2><p>从关注项进入时会直接选中对应任务。</p></div><AlertTriangle size={18} aria-hidden /></div>
      <EditorDesk embedded context={{ world: search.world, task: selectedTask }} />
    </Panel>
  </WorkbenchPageShell>
}
