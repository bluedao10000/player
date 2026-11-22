import { createServerFn } from '@tanstack/react-start'
import { discoveryCatalog } from '@anime/shared/knowledge'

// Only public chapter metadata crosses the server boundary; the guest's list stays on-device.
export const libraryCatalog = createServerFn({ method: 'GET' }).handler(
  async () => {
    try {
      const { items } = await discoveryCatalog()
      return items.flatMap((w) =>
        w.chapters.map((c) => ({
          href: c.href,
          title: c.title,
          work: w.title,
          edition: w.edition_label,
          label: w.format === 'film' ? '全片导读' : c.label,
          image: c.image || w.hero_asset_key,
        })),
      )
    } catch (error) {
      console.error(
        'Unable to load published chapters for the guest shelf',
        error,
      )
      return null
    }
  },
)
