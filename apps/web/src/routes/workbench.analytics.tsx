import { createFileRoute } from '@tanstack/react-router'
import { BackupReport } from '../features/workbench/backup-report'
import { ProductionReport } from '../features/workbench/production-report'
import { readAnalyticsFeed } from '../features/workbench/feed'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/analytics')({
  loader: ({ context }) => context.hasWorkbenchSession ? readAnalyticsFeed() : null,
  component: AnalyticsPage,
})

function AnalyticsPage() {
  const feed = Route.useLoaderData()
  const refresh = useWorkbenchRefresh()
  if (!feed) return null
  return <WorkbenchPageShell current="analytics" title="运营分析" english="ANALYTICS" description="查看制作用量和备份回执。趋势分析将在 Phase 4 提供。"
    updatedAt={feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <ProductionReport works={feed.usage} />
    <BackupReport data={feed.backup} checked={feed.checked} />
  </WorkbenchPageShell>
}
