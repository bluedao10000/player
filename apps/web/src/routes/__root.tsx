import { siteName } from '../site'
import {
  createRootRoute,
  HeadContent,
  Outlet,
  Scripts,
} from '@tanstack/react-router'
import postCss from '../features/post/post.css?url'
import type { ReactNode } from 'react'

export const Route = createRootRoute({
  ssr: true,
  head: () => ({
    links: [
      { rel: 'stylesheet', href: postCss },
      { rel: 'icon', type: 'image/svg+xml', sizes: 'any', href: '/brand/folio-plane.svg' },
      { rel: 'apple-touch-icon', sizes: '180x180', href: '/brand/apple-touch-icon.png' },
    ],
    meta: [
      { charSet: 'utf-8' },
      { title: siteName },
      { property: 'og:site_name', content: siteName },
      {
        name: 'viewport',
        content: 'width=device-width, initial-scale=1, viewport-fit=cover',
      },
    ],
  }),
  component: Outlet,
  shellComponent: Document,
})

function Document({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN">
      <head>
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  )
}
