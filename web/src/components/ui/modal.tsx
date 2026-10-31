import { createPortal } from 'react-dom'
import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react'

import { Icon } from './icon'

type ModalProps = {
  open: boolean
  onClose: () => void
  title: string
  description?: string
  children?: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg'
  rootClassName?: string
  closeLabel: string
}

const sizeClasses = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-3xl',
}

const modalStack: string[] = []
// Overlapping exits share the original background state, regardless of close order.
const backgroundLocks = new Map<HTMLElement, { count: number; inert: boolean; ariaHidden: string | null }>()
let originalOverflow = ''
let originalFocus: HTMLElement | null = null
const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function Modal({ open, onClose, title, description, children, footer, size = 'md', closeLabel, rootClassName = '' }: ModalProps) {
  const titleId = useId()
  const descriptionId = useId()
  const modalId = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)
  const openRef = useRef(open)
  const backdropPress = useRef<{ x: number; y: number } | null>(null)
  const [mounted, setMounted] = useState(open)
  const [closing, setClosing] = useState(false)

  useLayoutEffect(() => {
    onCloseRef.current = onClose
    openRef.current = open
  }, [onClose, open])

  useEffect(() => {
    if (open) {
      setMounted(true)
      setClosing(false)
      return
    }
    if (!mounted) return
    setClosing(true)
    const timer = window.setTimeout(() => {
      setMounted(false)
      setClosing(false)
    }, window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : 180)
    return () => window.clearTimeout(timer)
  }, [mounted, open])

  useLayoutEffect(() => {
    if (!mounted) return
    const dialog = dialogRef.current
    const previousFocus = document.activeElement as HTMLElement | null
    if (modalStack.length === 0) {
      originalOverflow = document.body.style.overflow
      originalFocus = previousFocus
    }
    document.querySelector('[aria-owns="imagesilo-notifications"]')?.removeAttribute('aria-owns')
    dialog?.setAttribute('aria-owns', 'imagesilo-notifications')
    modalStack.push(modalId)
    document.body.style.overflow = 'hidden'
    const background = Array.from(document.body.children)
      .filter((element) => element !== rootRef.current && !element.hasAttribute('data-modal-notifications')) as HTMLElement[]
    for (const element of background) {
      const lock = backgroundLocks.get(element) ?? {
        count: 0, inert: Boolean(element.inert), ariaHidden: element.getAttribute('aria-hidden'),
      }
      lock.count += 1
      backgroundLocks.set(element, lock)
      element.inert = true
      element.setAttribute('aria-hidden', 'true')
    }
    const timer = window.setTimeout(() => {
      const candidates = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])
      const initial =
        candidates.find((element) => element.dataset.modalInitialFocus !== undefined) ??
        candidates.find((element) => !element.classList.contains('ui-modal-close'))
      initial?.focus()
      if (!initial) dialogRef.current?.focus()
    }, 0)
    const handleKeyDown = (event: KeyboardEvent) => {
      if (modalStack.at(-1) !== modalId) return
      if (event.target instanceof Element && event.target.closest('[data-modal-notifications]') && event.key === 'Escape') return
      if (event.key === 'Escape' && openRef.current) {
        event.preventDefault()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return
      // Notifications belong to the active dialog's accessible interaction region.
      const notifications = Array.from(document.querySelectorAll<HTMLElement>('[data-modal-notifications] [data-sonner-toast]:not([data-removed="true"]) button:not([disabled])'))
      const candidates = [...Array.from(dialogRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? []), ...notifications]
      if (candidates.length === 0) {
        event.preventDefault()
        dialogRef.current?.focus()
        return
      }
      const index = candidates.indexOf(document.activeElement as HTMLElement)
      const next = index < 0 ? (event.shiftKey ? candidates.length - 1 : 0) : (index + (event.shiftKey ? -1 : 1) + candidates.length) % candidates.length
      event.preventDefault()
      candidates[next].focus()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      window.clearTimeout(timer)
      const wasTop = modalStack.at(-1) === modalId
      const stackIndex = modalStack.lastIndexOf(modalId)
      if (stackIndex >= 0) modalStack.splice(stackIndex, 1)
      document.removeEventListener('keydown', handleKeyDown)
      dialog?.removeAttribute('aria-owns')
      if (wasTop && modalStack.length > 0) {
        document.querySelector<HTMLElement>(`[data-modal-id="${modalStack.at(-1)}"]`)?.setAttribute('aria-owns', 'imagesilo-notifications')
      }
      for (const element of background) {
        const lock = backgroundLocks.get(element)
        if (!lock || --lock.count > 0) continue
        element.inert = lock.inert
        if (lock.ariaHidden === null) element.removeAttribute('aria-hidden')
        else element.setAttribute('aria-hidden', lock.ariaHidden)
        backgroundLocks.delete(element)
      }
      if (modalStack.length === 0) {
        document.body.style.overflow = originalOverflow
        if (originalFocus?.isConnected) originalFocus.focus()
        originalFocus = null
      } else if (wasTop && previousFocus?.isConnected) {
        previousFocus.focus()
      }
    }
  }, [modalId, mounted])

  if (!mounted) return null

  return createPortal(
    <div
      ref={rootRef}
      className={`ui-modal-root ${rootClassName}`}
      data-state={closing || !open ? 'closed' : 'open'}
      role="presentation"
      onPointerDown={(event) => {
        const root = event.currentTarget
        const rect = root.getBoundingClientRect()
        const inContent = event.clientX < rect.left + root.clientWidth && event.clientY < rect.top + root.clientHeight
        backdropPress.current = event.target === root && inContent && event.button === 0 ? { x: event.clientX, y: event.clientY } : null
      }}
      onPointerCancel={() => { backdropPress.current = null }}
      onClick={(event) => {
        const press = backdropPress.current
        backdropPress.current = null
        if (open && modalStack.at(-1) === modalId && event.target === event.currentTarget && press && Math.hypot(event.clientX - press.x, event.clientY - press.y) < 8) onCloseRef.current()
      }}
    >
      <div
        ref={dialogRef}
        className={`ui-modal-panel ${sizeClasses[size]}`}
        role="dialog"
        data-modal-id={modalId}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        tabIndex={-1}
      >
        <button className="ui-modal-close" type="button" aria-label={closeLabel} onClick={() => onCloseRef.current()}>
          <Icon name="x" className="h-5 w-5" />
        </button>
        <header className="ui-modal-header">
          <h2 id={titleId}>{title}</h2>
          {description ? <p id={descriptionId}>{description}</p> : null}
        </header>
        {children ? <div className="ui-modal-body">{children}</div> : null}
        {footer ? <footer className="ui-modal-footer">{footer}</footer> : null}
      </div>
    </div>,
    document.body,
  )
}
