import { createFileRoute } from '@tanstack/react-router'
import { readKnowledgeWorkFeed } from '../features/workbench/feed'
import { KnowledgeWorkDetail } from '../features/workbench/knowledge-detail'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/knowledge/$work')({
  loader: ({ context, params }) => context.hasWorkbenchSession ? readKnowledgeWorkFeed({ data: params.work }) : null,
  component: WorkDetailPage,
})

function WorkDetailPage() {
  const feed = Route.useLoaderData()
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  if (!feed) return null
  const detail = feed.detail
  return <WorkbenchPageShell current="knowledge" title={detail?.overview.title ?? '作品知识详情'} english="KNOWLEDGE DETAIL"
    description="查看作品知识版本、完整度分项与缺项。" updatedAt={feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto}
    refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <KnowledgeWorkDetail feed={feed} candidate={search.candidate} />
  </WorkbenchPageShell>
}
