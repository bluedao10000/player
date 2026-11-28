import type { ChapterDetail } from '@anime/shared/knowledge'

// An explicitly empty published edition is authoritative, not a request for fallback.
export function chapterReading({ reader_passages, research, blocks }: ChapterDetail) {
  if (reader_passages != null) return { kind: 'published' as const, blocks: reader_passages }
  if (!research) return { kind: 'raw' as const, blocks }
  return {
    kind: 'research' as const,
    paragraphs: research.paragraphs.map(({ text }, i) => ({ key: `research-${i + 1}`, text, anchors: [] })),
    blocks: blocks.filter(block => block.anchors.length > 0),
  }
}
