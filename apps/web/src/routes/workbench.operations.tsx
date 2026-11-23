import { createFileRoute } from '@tanstack/react-router'
import { AttentionList } from '../features/workbench/dashboard/overview'
import { readOperationsFeed, readOpsAttention, readOpsTopology } from '../features/workbench/feed'
import { LiveTerminal } from '../features/workbench/terminal/live-terminal'
import { WorkflowCanvas } from '../features/workbench/workflow/workflow-canvas'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/operations')({
  loaderDeps: ({ search }) => ({ window: search.window ?? '24h', world: typeof search.world === 'string' ? search.world : '' }),
  loader: async ({ context, deps }) => context.hasWorkbenchSession
    ? Promise.all([
        readOperationsFeed(),
        readOpsAttention(),
        readOpsTopology({ data: { window: deps.window, world: deps.world } }),
      ]).then(([feed, attention, topology]) => ({ feed, attention, topology }))
    : null,
  component: OperationsPage,
})

function OperationsPage() {
  const data = Route.useLoaderData()
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  if (!data?.feed) return null
  const { feed, attention, topology } = data
  const selected = attention?.attention.filter((item) =>
    (search.worker && item.link.params.worker === search.worker && (!search.loop || item.link.params.loop === search.loop)) ||
    (search.run_key && item.link.params.run_key === search.run_key) ||
    (search.source_id && item.link.params.source_id === search.source_id),
  ) ?? []
  const initialStage = typeof search.stage === 'string' ? search.stage : ''
  return <WorkbenchPageShell current="operations" title="运营拓扑" english="OPERATIONS" description="按阶段查看当前队列、窗口执行结果、工作进程与 Agent 执行实例。"
    updatedAt={topology?.as_of ?? feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    {selected.length > 0 && <AttentionList items={selected} title="选中的运行事项" />}
    <WorkflowCanvas initialData={topology} worlds={feed.worlds ?? []} initialWorld={typeof search.world === 'string' ? search.world : ''}
      initialWindow={search.window ?? '24h'} initialStage={initialStage} />
    <details className="wb-live-terminal" open={Boolean(search.run_key)}>
      <summary><span>实时运行日志</span><small>来源同步与质量扫描 · 非进程 stdout / stderr</small></summary>
      <LiveTerminal runs={feed.runs} syncOnline={feed.syncOnline} qualityOnline={feed.qualityOnline} auto={refresh.auto}
        onAutoChange={refresh.setAuto} initialRunKey={search.run_key} />
    </details>
  </WorkbenchPageShell>
}
