import { useEffect, useRef, type ReactNode } from 'react'

type DropdownMenuProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  trigger: ReactNode
  children: ReactNode
  className?: string
  rootClassName?: string
  align?: 'left' | 'right'
}

export function DropdownMenu({ open, onOpenChange, trigger, children, className = '', rootClassName = '', align = 'right' }: DropdownMenuProps) {
  const rootRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: Event) => {
      if (!rootRef.current?.contains(event.target as Node)) onOpenChange(false)
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onOpenChange(false)
    }
    document.addEventListener('pointerdown', handleOutside)
    document.addEventListener('mousedown', handleOutside)
    document.addEventListener('touchstart', handleOutside)
    document.addEventListener('click', handleOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('pointerdown', handleOutside)
      document.removeEventListener('mousedown', handleOutside)
      document.removeEventListener('touchstart', handleOutside)
      document.removeEventListener('click', handleOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [onOpenChange, open])

  return (
    <div className={`ui-dropdown-root ${rootClassName}`.trim()} ref={rootRef}>
      {trigger}
      {open ? <div className={`ui-dropdown-panel ${align === 'left' ? 'is-left' : ''} ${className}`.trim()}>{children}</div> : null}
    </div>
  )
}

export function DropdownItem({ children, onClick, active = false, destructive = false }: {
  children: ReactNode
  onClick: () => void
  active?: boolean
  destructive?: boolean
}) {
  return <button className="ui-dropdown-item" data-active={active || undefined} data-destructive={destructive || undefined} type="button" onClick={onClick}>{children}</button>
}
