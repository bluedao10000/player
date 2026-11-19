import type { ReactNode } from 'react'
export type IconName =
  | 'spark'
  | 'forward'
  | 'moon'
  | 'user'
  | 'left'
  | 'right'
  | 'map'
  | 'wave'
  | 'book'
  | 'bookmark'
  | 'arrow'
  | 'pin'
  | 'feather'
  | 'mail'
  | 'compass'
  | 'people'
  | 'clock'
  | 'back'
  | 'check'
const paths: Record<IconName, ReactNode> = {
  spark: <path d="m12 2 2.7 7.3L22 12l-7.3 2.7L12 22l-2.7-7.3L2 12l7.3-2.7z" />,
  forward: <path d="M6 18 18 6M6 6h12v12" />,
  moon: <path d="M20 14A8.7 8.7 0 0 1 10 3a9 9 0 1 0 10 11Z" />,
  user: (
    <>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21a8 8 0 0 1 16 0" />
    </>
  ),
  left: <path d="m15 5-7 7 7 7" />,
  right: <path d="m9 5 7 7-7 7" />,
  map: (
    <>
      <path d="m2 5 6-2 8 3 6-2v15l-6 2-8-3-6 2zM8 3v15m8-12v15" />
    </>
  ),
  wave: (
    <>
      <path d="M2 15c4 2 7 2 10-1 3-3 5-7 9-7-1 3-1 5 1 6-4 0-5-2-5-3M2 19c5 2 8 1 11-1 3-2 5-1 9 0" />
      <circle cx="6" cy="7" r="2" />
    </>
  ),
  book: (
    <>
      <path d="M3 5c3-1 6-1 9 1 3-2 6-2 9-1v14c-3-1-6-1-9 1-3-2-6-2-9-1zM12 6v14" />
    </>
  ),
  bookmark: <path d="M6 3h12v18l-6-4-6 4z" />,
  arrow: <path d="M4 12h15m-6-6 6 6-6 6" />,
  pin: (
    <>
      <path d="M19 10c0 5-7 11-7 11S5 15 5 10a7 7 0 1 1 14 0Z" />
      <circle cx="12" cy="10" r="2" />
    </>
  ),
  feather: (
    <>
      <path d="m3 21 13-13m-9 9c-2-7 2-13 14-14 0 9-5 16-14 14Z" />
    </>
  ),
  mail: (
    <>
      <rect x="2" y="5" width="20" height="14" rx="1" />
      <path d="m3 6 9 7 9-7" />
    </>
  ),
  compass: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="m15 8-2 6-5 2 2-6z" />
    </>
  ),
  people: (
    <>
      <circle cx="10" cy="8" r="3.5" />
      <path d="M3 20c.5-7 13-7 14 0M16 5c4 0 4 6 1 7m1 3c2 0 3 2 3 4" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 6v6l4 2" />
    </>
  ),
  back: <path d="m10 6-6 6 6 6M4 12h16" />,
  check: <path d="m4 12 5 5L20 6" />,
}
export function Icon({ name, size = 18 }: { name: IconName; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.65"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  )
}
