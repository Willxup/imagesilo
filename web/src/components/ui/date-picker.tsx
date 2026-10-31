import { useEffect, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'

import { cn } from '../../lib/utils'
import { Icon } from './icon'

type DatePickerProps = {
  id?: string
  name?: string
  value: string
  onChange: (value: string) => void
  locale: string
  ariaLabel: string
  placeholder: string
  clearLabel: string
  todayLabel: string
  previousMonthLabel: string
  nextMonthLabel: string
  min?: string
  className?: string
}

export function DatePicker({
  id,
  name,
  value,
  onChange,
  locale,
  ariaLabel,
  placeholder,
  clearLabel,
  todayLabel,
  previousMonthLabel,
  nextMonthLabel,
  min,
  className,
}: DatePickerProps) {
  const rootRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const returnFocus = () => rootRef.current?.querySelector<HTMLButtonElement>('.ui-date-trigger')?.focus()
  const selected = parseDate(value)
  const minimum = parseDate(min ?? '')
  const [open, setOpen] = useState(false)
  const [focusedDate, setFocusedDate] = useState(() => toDateValue(selected ?? new Date()))
  const focusDay = useRef(false)
  const [visibleMonth, setVisibleMonth] = useState(() => startOfMonth(selected ?? new Date()))

  useEffect(() => {
    if (!open) return
    const closeOutside = (event: Event) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && rootRef.current?.contains(event.target as Node)) {
        event.preventDefault()
        event.stopPropagation()
        setOpen(false)
        returnFocus()
      }
    }
    document.addEventListener('pointerdown', closeOutside)
    document.addEventListener('focusin', closeOutside)
    document.addEventListener('keydown', closeOnEscape)
    return () => {
      document.removeEventListener('pointerdown', closeOutside)
      document.removeEventListener('focusin', closeOutside)
      document.removeEventListener('keydown', closeOnEscape)
    }
  }, [open])

  useLayoutEffect(() => {
    if (!open) return
    const position = () => {
      const root = rootRef.current
      const panel = panelRef.current
      if (!root || !panel) return
      panel.style.maxHeight = ''
      const rect = root.getBoundingClientRect()
      const x = Math.max(12, Math.min(rect.left, window.innerWidth - panel.offsetWidth - 12))
      const above = Math.max(0, rect.top - 20)
      const below = Math.max(0, window.innerHeight - rect.bottom - 20)
      const opensAbove = below < panel.offsetHeight && above > below
      panel.style.left = `${x - rect.left}px`
      panel.style.top = opensAbove ? 'auto' : 'calc(100% + 8px)'
      panel.style.bottom = opensAbove ? 'calc(100% + 8px)' : 'auto'
      panel.style.maxHeight = `${Math.max(44, opensAbove ? above : below)}px`
    }
    position()
    const onScroll = (event: Event) => {
      if (!panelRef.current?.contains(event.target as Node)) position()
    }
    window.addEventListener('resize', position)
    window.addEventListener('scroll', onScroll, true)
    return () => {
      window.removeEventListener('resize', position)
      window.removeEventListener('scroll', onScroll, true)
    }
  }, [open])

  useLayoutEffect(() => {
    if (open && focusDay.current) {
      const panel = panelRef.current
      const day = panel?.querySelector<HTMLButtonElement>(`[data-date="${focusedDate}"]`)
      day?.focus({ preventScroll: true })
      if (panel && day) {
        // Reveal keyboard focus within the calendar without moving the page behind it.
        const top = panel.getBoundingClientRect().top + panel.clientTop
        const bottom = top + panel.clientHeight
        const rect = day.getBoundingClientRect()
        if (rect.top < top) panel.scrollTop += rect.top - top
        else if (rect.bottom > bottom) panel.scrollTop += rect.bottom - bottom
      }
      focusDay.current = false
    }
  }, [open, focusedDate, visibleMonth])

  function moveFocus(date: Date) {
    const next = minimum && startOfDay(date) < startOfDay(minimum) ? minimum : date
    focusDay.current = true
    setFocusedDate(toDateValue(next))
    setVisibleMonth(startOfMonth(next))
  }

  function navigateDate(event: ReactKeyboardEvent<HTMLButtonElement>, date: Date) {
    const offsets: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7, Home: -date.getDay(), End: 6 - date.getDay() }
    if (event.key in offsets) {
      event.preventDefault()
      moveFocus(new Date(date.getFullYear(), date.getMonth(), date.getDate() + offsets[event.key]))
    } else if (event.key === 'PageUp' || event.key === 'PageDown') {
      event.preventDefault()
      const month = addMonths(date, (event.key === 'PageUp' ? -1 : 1) * (event.shiftKey ? 12 : 1))
      const lastDay = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()
      moveFocus(new Date(month.getFullYear(), month.getMonth(), Math.min(date.getDate(), lastDay)))
    }
  }

  function changeMonth(amount: number) {
    const month = addMonths(visibleMonth, amount)
    const firstEnabled = minimum && startOfDay(month) < startOfDay(minimum) ? minimum : month
    setVisibleMonth(month)
    setFocusedDate(toDateValue(firstEnabled))
  }

  const days = useMemo(() => calendarDays(visibleMonth), [visibleMonth])
  const weekdays = useMemo(() => weekdayLabels(locale), [locale])
  const dateFormatter = useMemo(() => new Intl.DateTimeFormat(locale, { year: 'numeric', month: 'short', day: 'numeric' }), [locale])
  const accessibleFormatter = useMemo(() => new Intl.DateTimeFormat(locale, { dateStyle: 'full' }), [locale])
  const monthFormatter = useMemo(() => new Intl.DateTimeFormat(locale, { year: 'numeric', month: 'long' }), [locale])

  function selectDate(date: Date) {
    if (minimum && startOfDay(date) < startOfDay(minimum)) return
    onChange(toDateValue(date))
    setVisibleMonth(startOfMonth(date))
    setOpen(false)
    returnFocus()
  }

  function showPicker() {
    if (!open) moveFocus(selected ?? new Date())
    setOpen((current) => !current)
  }

  return (
    <div className={cn('ui-date-picker', className)} ref={rootRef}>
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <button
        id={id}
        className="ui-date-trigger"
        data-placeholder={!selected || undefined}
        type="button"
        aria-label={`${ariaLabel}: ${selected ? dateFormatter.format(selected) : placeholder}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={showPicker}
      >
        <Icon name="calendar" />
        <span>{selected ? dateFormatter.format(selected) : placeholder}</span>
        <Icon name="chevronDown" />
      </button>
      {open ? (
        <div ref={panelRef} className="ui-date-popover" role="dialog" aria-label={ariaLabel}>
          <div className="ui-date-header">
            <button type="button" aria-label={previousMonthLabel} disabled={Boolean(minimum && startOfMonth(minimum) >= visibleMonth)} onClick={() => changeMonth(-1)}><Icon name="chevronLeft" /></button>
            <strong aria-live="polite">{monthFormatter.format(visibleMonth)}</strong>
            <button type="button" aria-label={nextMonthLabel} onClick={() => changeMonth(1)}><Icon name="chevronRight" /></button>
          </div>
          <div className="ui-date-weekdays" aria-hidden="true">
            {weekdays.map((weekday, index) => <span key={`${weekday}-${index}`}>{weekday}</span>)}
          </div>
          <div className="ui-date-grid">
            {days.map((date) => {
              const dateValue = toDateValue(date)
              const disabled = Boolean(minimum && startOfDay(date) < startOfDay(minimum))
              return (
                <button
                  key={dateValue}
                  type="button"
                  aria-label={accessibleFormatter.format(date)}
                  aria-pressed={dateValue === value}
                  data-date={dateValue}
                  tabIndex={dateValue === focusedDate ? 0 : -1}
                  onKeyDown={(event) => navigateDate(event, date)}
                  onFocus={() => setFocusedDate(dateValue)}
                  disabled={disabled}
                  data-outside={date.getMonth() !== visibleMonth.getMonth() || undefined}
                  data-selected={dateValue === value || undefined}
                  data-today={dateValue === toDateValue(new Date()) || undefined}
                  onClick={() => selectDate(date)}
                >
                  {date.getDate()}
                </button>
              )
            })}
          </div>
          <div className="ui-date-footer">
            <button type="button" onClick={() => selectDate(new Date())} disabled={Boolean(minimum && startOfDay(new Date()) < startOfDay(minimum))}>{todayLabel}</button>
            <button type="button" onClick={() => { onChange(''); setOpen(false); returnFocus() }} disabled={!value}>{clearLabel}</button>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function parseDate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return null
  const result = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  if (result.getFullYear() !== Number(match[1]) || result.getMonth() !== Number(match[2]) - 1 || result.getDate() !== Number(match[3])) return null
  return result
}

function toDateValue(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function startOfDay(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
}

function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1)
}

function addMonths(date: Date, amount: number) {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1)
}

function calendarDays(month: Date) {
  const start = new Date(month.getFullYear(), month.getMonth(), 1 - month.getDay())
  return Array.from({ length: 42 }, (_, index) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + index))
}

function weekdayLabels(locale: string) {
  const formatter = new Intl.DateTimeFormat(locale, { weekday: 'narrow' })
  const sunday = new Date(2024, 0, 7)
  return Array.from({ length: 7 }, (_, index) => formatter.format(new Date(2024, 0, sunday.getDate() + index)))
}
