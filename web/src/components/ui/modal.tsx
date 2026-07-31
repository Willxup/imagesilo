import { createPortal } from 'react-dom'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'

import { Icon } from './icon'

type ModalProps = {
  open: boolean
  onClose: () => void
  title: string
  description?: string
  children?: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg'
  closeLabel: string
}

const sizeClasses = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-3xl',
}

const modalStack: string[] = []
const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function Modal({ open, onClose, title, description, children, footer, size = 'md', closeLabel }: ModalProps) {
  const titleId = useId()
  const descriptionId = useId()
  const modalId = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)
  const openRef = useRef(open)
  const [mounted, setMounted] = useState(open)
  const [closing, setClosing] = useState(false)

  useEffect(() => {
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
    }, 180)
    return () => window.clearTimeout(timer)
  }, [mounted, open])

  useEffect(() => {
    if (!mounted) return
    modalStack.push(modalId)
    const previousFocus = document.activeElement as HTMLElement | null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const background = Array.from(document.body.children)
      .filter((element) => element !== rootRef.current)
      .map((element) => ({
        element: element as HTMLElement,
        inert: Boolean((element as HTMLElement).inert),
        ariaHidden: element.getAttribute('aria-hidden'),
      }))
    for (const item of background) {
      item.element.inert = true
      item.element.setAttribute('aria-hidden', 'true')
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
      if (event.key === 'Escape' && openRef.current) {
        event.preventDefault()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const candidates = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])
      if (candidates.length === 0) {
        event.preventDefault()
        dialogRef.current?.focus()
        return
      }
      const first = candidates[0]
      const last = candidates[candidates.length - 1]
      if (event.shiftKey && (document.activeElement === first || !dialogRef.current?.contains(document.activeElement))) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (document.activeElement === last || !dialogRef.current?.contains(document.activeElement))) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      window.clearTimeout(timer)
      const stackIndex = modalStack.lastIndexOf(modalId)
      if (stackIndex >= 0) modalStack.splice(stackIndex, 1)
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', handleKeyDown)
      for (const item of background) {
        item.element.inert = item.inert
        if (item.ariaHidden === null) item.element.removeAttribute('aria-hidden')
        else item.element.setAttribute('aria-hidden', item.ariaHidden)
      }
      previousFocus?.focus()
    }
  }, [modalId, mounted])

  if (!mounted) return null

  return createPortal(
    <div
      ref={rootRef}
      className="ui-modal-root"
      data-state={closing || !open ? 'closed' : 'open'}
      role="presentation"
      onMouseDown={(event) => {
        if (open && modalStack.at(-1) === modalId && event.target === event.currentTarget) onCloseRef.current()
      }}
    >
      <div
        ref={dialogRef}
        className={`ui-modal-panel ${sizeClasses[size]}`}
        role="dialog"
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
