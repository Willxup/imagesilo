import { useEffect, useMemo, useRef, useState } from 'react'

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
  const selected = parseDate(value)
  const minimum = parseDate(min ?? '')
  const [open, setOpen] = useState(false)
  const [visibleMonth, setVisibleMonth] = useState(() => startOfMonth(selected ?? new Date()))

  useEffect(() => {
    if (!open) return
    const closeOutside = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('pointerdown', closeOutside)
    document.addEventListener('keydown', closeOnEscape)
    return () => {
      document.removeEventListener('pointerdown', closeOutside)
      document.removeEventListener('keydown', closeOnEscape)
    }
  }, [open])

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
  }

  function showPicker() {
    setVisibleMonth(startOfMonth(selected ?? new Date()))
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
        aria-label={ariaLabel}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={showPicker}
      >
        <Icon name="calendar" />
        <span>{selected ? dateFormatter.format(selected) : placeholder}</span>
        <Icon name="chevronDown" />
      </button>
      {open ? (
        <div className="ui-date-popover" role="dialog" aria-label={ariaLabel}>
          <div className="ui-date-header">
            <button type="button" aria-label={previousMonthLabel} onClick={() => setVisibleMonth((month) => addMonths(month, -1))}><Icon name="chevronLeft" /></button>
            <strong>{monthFormatter.format(visibleMonth)}</strong>
            <button type="button" aria-label={nextMonthLabel} onClick={() => setVisibleMonth((month) => addMonths(month, 1))}><Icon name="chevronRight" /></button>
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
            <button type="button" onClick={() => { onChange(''); setOpen(false) }} disabled={!value}>{clearLabel}</button>
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
