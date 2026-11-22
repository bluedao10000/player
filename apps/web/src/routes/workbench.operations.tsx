import { createFileRoute } from '@tanstack/react-router'
import { AttentionList } from '../features/workbench/dashboard/overview'
import { readOperationsFeed, readOpsAttention } from '../features/workbench/feed'
import { LiveTerminal } from '../features/workbench/terminal/live-terminal'
import { WorkflowCanvas } from '../features/workbench/workflow/workflow-canvas'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/operations')({
  loader: async ({ context }) => context.hasWorkbenchSession
    ? Promise.all([readOperationsFeed(), readOpsAttention()]).then(([feed, attention]) => ({ feed, attention }))
    : null,
  component: OperationsPage,
})

function OperationsPage() {
  const data = Route.useLoaderData()
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  if (!data?.feed) return null
  const { feed, attention } = data
  const selected = attention?.attention.filter((item) =>
    (search.worker && item.link.params.worker === search.worker && (!search.loop || item.link.params.loop === search.loop)) ||
    (search.run_key && item.link.params.run_key === search.run_key) ||
    (search.source_id && item.link.params.source_id === search.source_id),
  ) ?? []
  return <WorkbenchPageShell current="operations" title="运营拓扑" english="OPERATIONS" description="查看现有工作流拓扑与结构化运行日志；拓扑计数口径将在 Phase 3 更新。"
    updatedAt={feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    {selected.length > 0 && <AttentionList items={selected} title="选中的运行事项" />}
    <WorkflowCanvas feed={feed} />
    <LiveTerminal runs={feed.runs} syncOnline={feed.syncOnline} qualityOnline={feed.qualityOnline} auto={refresh.auto}
      onAutoChange={refresh.setAuto} initialRunKey={search.run_key} />
  </WorkbenchPageShell>
}
