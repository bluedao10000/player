import { discoveryCatalog } from '@anime/shared/knowledge'

// The shelf reads public chapter metadata; the guest's list stays on-device.
export const libraryCatalog = async () => {
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
}
