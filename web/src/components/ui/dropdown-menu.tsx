import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'

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
  const panelRef = useRef<HTMLDivElement>(null)
  const [mounted, setMounted] = useState(open)
  const [closing, setClosing] = useState(false)

  useEffect(() => {
    if (open) {
      setMounted(true)
      setClosing(false)
      return
    }
    if (!mounted) return
    setClosing(true)
    const timer = window.setTimeout(() => { setMounted(false); setClosing(false) }, window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : 140)
    return () => window.clearTimeout(timer)
  }, [mounted, open])

  useLayoutEffect(() => {
    if (!open || !mounted) return
    const position = () => {
      const root = rootRef.current
      const panel = panelRef.current
      if (!root || !panel) return
      // Measure natural height before choosing a side, then constrain only if neither side fits.
      panel.style.top = ''
      panel.style.bottom = ''
      panel.style.maxHeight = ''
      const preferAbove = getComputedStyle(panel).bottom !== 'auto'
      const rect = root.getBoundingClientRect()
      const width = panel.offsetWidth
      const height = panel.offsetHeight
      const desired = align === 'left' ? rect.left : rect.right - width
      const viewportX = Math.max(12, Math.min(desired, window.innerWidth - width - 12))
      panel.style.left = `${viewportX - rect.left}px`
      panel.style.right = 'auto'
      const above = Math.max(0, rect.top - 24)
      const below = Math.max(0, window.innerHeight - rect.bottom - 20)
      const opensAbove = preferAbove ? above >= height || above > below : below < height && above > below
      panel.style.top = opensAbove ? 'auto' : 'calc(100% + 8px)'
      panel.style.bottom = opensAbove ? 'calc(100% + 12px)' : 'auto'
      panel.style.maxHeight = `${Math.max(44, opensAbove ? above : below)}px`
      panel.style.setProperty('--menu-enter-y', opensAbove ? '4px' : '-4px')
    }
    position()
    window.addEventListener('resize', position)
    return () => window.removeEventListener('resize', position)
  }, [align, mounted, open])

  useEffect(() => {
    if (!open) return
    const handleOutside = (event: Event) => {
      if (!rootRef.current?.contains(event.target as Node)) onOpenChange(false)
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) return
      if (event.key === 'Escape') {
        event.preventDefault()
        onOpenChange(false)
        rootRef.current?.querySelector<HTMLElement>('button')?.focus()
        return
      }
      if (event.key === 'Tab') { onOpenChange(false); return }
      if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const items = Array.from(panelRef.current?.querySelectorAll<HTMLButtonElement>('.ui-dropdown-item') ?? [])
      if (items.length === 0) return
      const current = items.indexOf(document.activeElement as HTMLButtonElement)
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : current < 0 ? (event.key === 'ArrowDown' ? 0 : items.length - 1) : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
      items[next]?.focus()
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
      {mounted ? <div ref={panelRef} className={`ui-dropdown-panel ${align === 'left' ? 'is-left' : ''} ${className}`.trim()} data-state={closing || !open ? 'closed' : 'open'} inert={!open || undefined} role="menu">{children}</div> : null}
    </div>
  )
}

export function DropdownItem({ children, onClick, active = false, destructive = false }: {
  children: ReactNode
  onClick: () => void
  active?: boolean
  destructive?: boolean
}) {
  return <button className="ui-dropdown-item" data-active={active || undefined} data-destructive={destructive || undefined} role="menuitem" type="button" onClick={(event) => { event.currentTarget.closest('.ui-dropdown-root')?.querySelector<HTMLElement>(':scope > button')?.focus(); onClick() }}>{children}</button>
}
