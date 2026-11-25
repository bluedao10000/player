import { createRoot } from 'react-dom/client'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { routeTree } from './routeTree.public.gen.js'

const router = createRouter({ routeTree, scrollRestoration: true })
createRoot(document.getElementById('app')!).render(<RouterProvider router={router} />)
