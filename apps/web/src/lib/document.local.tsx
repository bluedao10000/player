import { Scripts } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import postCss from '../features/post/post.css?url'

export function Document({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
        <link rel="stylesheet" href={postCss} />
      </head>
      <body>{children}<Scripts /></body>
    </html>
  )
}
