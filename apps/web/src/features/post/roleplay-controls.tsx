import type { ReactNode } from 'react'
import { Icon } from './icons'

export function Narration({ children }: { children: ReactNode }) {
  return <p>{children}</p>
}
export function Dialogue({ text }: { text: string }) {
  return <p className="standfirst">{text}</p>
}
export function ChoiceButton({
  children,
  disabled,
  onClick,
}: {
  children: ReactNode
  disabled: boolean
  onClick: () => void
}) {
  return (
    <button disabled={disabled} onClick={onClick}>
      {children}
      <Icon name="arrow" />
    </button>
  )
}
export function RoleplayComposer({
  options,
  ready,
  onChoose,
}: {
  options: readonly string[]
  ready: boolean
  onChoose: (index: number) => void
}) {
  return (
    <>
      <h2>你打算怎么做？</h2>
      <div className="choices">
        {options.map((label, i) => (
          <ChoiceButton
            key={label}
            disabled={!ready}
            onClick={() => onChoose(i)}
          >
            {label}
          </ChoiceButton>
        ))}
      </div>
    </>
  )
}
