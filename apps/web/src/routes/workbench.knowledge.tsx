import { createFileRoute } from '@tanstack/react-router'
import { MediaGallery } from '../features/workbench/media-gallery'
import { ScopeReport } from '../features/workbench/scope-report'
import { WorkbenchPageShell, useWorkbenchRefresh } from '../features/workbench/shared/page'
import { WorldInventory } from '../features/workbench/world-inventory'
import { readKnowledgeFeed } from '../features/workbench/feed'

export const Route = createFileRoute('/workbench/knowledge')({
  loader: ({ context }) => context.hasWorkbenchSession ? readKnowledgeFeed() : null,
  component: KnowledgePage,
})

function KnowledgePage() {
  const feed = Route.useLoaderData()
  const search = Route.useSearch()
  const refresh = useWorkbenchRefresh()
  if (!feed) return null
  return <WorkbenchPageShell current="knowledge" title="知识资产" english="KNOWLEDGE" description="查看作品世界、分集验收范围和图片候选。"
    updatedAt={feed.checked} auto={refresh.auto} onAutoChange={refresh.setAuto} refreshing={refresh.refreshing} onRefresh={() => void refresh.refresh()}>
    <div className="wb-report-stack">
      <WorldInventory worlds={feed.worlds} scopes={feed.scopes} />
      <ScopeReport scopes={feed.scopes} />
      <MediaGallery key={search.candidate} items={feed.media} candidate={search.candidate} />
    </div>
  </WorkbenchPageShell>
}
