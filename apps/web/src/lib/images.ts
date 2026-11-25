import { bundledImages } from './images.generated'

export const imageOrigin = 'https://img.codeiy.com'

// PostgreSQL keeps archive references for publication and checksum verification.
const archivedImage = /^\/art\/(?:episode-scenes\/([a-f0-9]{64})\.jpg|portraits\/([a-f0-9]{64})\.webp)$/
const deliveryPath = /^\/i\/(?:[a-f0-9]{64}|[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12})$/

export function imageUrl(reference: string | null | undefined): string | undefined {
  if (!reference) return undefined
  const archived = archivedImage.exec(reference)
  const digest = Object.hasOwn(bundledImages, reference) ? bundledImages[reference] : archived?.[1] ?? archived?.[2]
  if (digest) return imageOrigin + '/i/' + digest

  if (!reference.startsWith('/i/') && !reference.startsWith(imageOrigin + '/i/')) return undefined
  try {
    const url = new URL(reference, imageOrigin)
    if (url.origin === imageOrigin && deliveryPath.test(url.pathname) && !url.hash)
      return url.href
  } catch {
    return undefined
  }
}

export function storyImage(name: string) {
  return imageUrl('/art/' + name + '.webp')
}
