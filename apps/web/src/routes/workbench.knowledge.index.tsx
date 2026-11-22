import { createFileRoute } from '@tanstack/react-router'
import { readKnowledgeCenterFeed } from '../features/workbench/feed'
import { KnowledgeCenter } from '../features/workbench/knowledge-center'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'

export const Route = createFileRoute('/workbench/knowledge/')({
  loader: ({ context }) => context.hasWorkbenchSession ? readKnowledgeCenterFeed() : null,
  component: KnowledgeCenterPage,
})

function KnowledgeCenterPage() {
  const feed = Route.useLoaderData()
  const refresh = useWorkbenchRefresh()
  if (!feed) return null
  return <WorkbenchPageShell current="knowledge" title="知识资产" english="KNOWLEDGE" description="查看全库知识统计、完整度分项和作品目录。"
    updatedAt={feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <KnowledgeCenter summary={feed.summary} works={feed.works} />
  </WorkbenchPageShell>
}
