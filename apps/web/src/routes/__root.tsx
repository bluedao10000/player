import { createRootRoute, HeadContent, Outlet } from '@tanstack/react-router'
import '../features/post/post.css'
import { Document } from '../lib/document'
import { siteName } from '../site'
import { brandMark } from '../lib/brand'
import { imageUrl } from '../lib/images'

export const Route = createRootRoute({
  head: () => ({
    links: [
      { rel: 'icon', type: 'image/svg+xml', sizes: 'any', href: brandMark },
      { rel: 'apple-touch-icon', sizes: '180x180', href: imageUrl('/brand/apple-touch-icon.png') },
    ],
    meta: [
      { title: siteName },
      { property: 'og:site_name', content: siteName },
    ],
  }),
  shellComponent: Document,
  component: () => <><HeadContent /><Outlet /></>,
})
