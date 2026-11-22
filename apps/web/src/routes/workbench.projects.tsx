import { createFileRoute } from '@tanstack/react-router'
import { ProductionPlans } from '../features/workbench/production-plans'
import { ProductionReport } from '../features/workbench/production-report'
import { ReleaseOperations } from '../features/workbench/dashboard/operations'
import { readProjectFeed, readOpsAttention } from '../features/workbench/feed'
import { AttentionList } from '../features/workbench/dashboard/overview'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/projects')({
  loader: async ({ context }) => context.hasWorkbenchSession
    ? Promise.all([readProjectFeed(), readOpsAttention()]).then(([feed, attention]) => ({ feed, attention }))
    : null,
  component: ProjectsPage,
})

function ProjectsPage() {
  const data = Route.useLoaderData()
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  if (!data?.feed) return null
  const { feed, attention } = data
  const selected = attention?.attention.filter((item) =>
    (search.source_id && item.link.params.source_id === search.source_id) ||
    (search.run_key && item.link.params.run_key === search.run_key) ||
    (search.item_key && item.link.params.item_key === search.item_key) ||
    (search.plan && item.link.params.plan === search.plan),
  ) ?? []
  return <WorkbenchPageShell current="projects" title="项目" english="PROJECTS" description="管理分集制作计划、补集准入、发布记录和作品用量。"
    updatedAt={feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <div className="wb-report-stack">
      {selected.length > 0 && <AttentionList items={selected} title="选中的项目事项" />}
      <ProductionPlans initialPlan={search.plan} />
      <ReleaseOperations feed={feed} />
      <ProductionReport works={feed.usage} />
    </div>
  </WorkbenchPageShell>
}
