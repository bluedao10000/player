import { createFileRoute } from '@tanstack/react-router'
import { Overview } from '../features/workbench/dashboard/overview'
import { readOpsOverview } from '../features/workbench/feed'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/')({
  loader: ({ context }) => context.hasWorkbenchSession ? readOpsOverview() : null,
  component: OverviewPage,
})

function OverviewPage() {
  const data = Route.useLoaderData()
  const refresh = useWorkbenchRefresh()
  return <WorkbenchPageShell current="overview" title="运营总览" english="OVERVIEW" description="查看作品、知识资产、工作进程和待处理事项。" updatedAt={data?.as_of ?? null}
    auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <Overview data={data} />
  </WorkbenchPageShell>
}
