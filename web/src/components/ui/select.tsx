import * as SelectPrimitive from '@radix-ui/react-select'
import { Check, ChevronDown, ChevronUp } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { cn } from '../../lib/utils'

export type SelectOption = {
  value: string
  label: string
  disabled?: boolean
}

type SelectProps = {
  options: SelectOption[]
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  name?: string
  id?: string
  ariaLabel?: string
  placeholder?: string
  disabled?: boolean
  className?: string
  size?: 'sm' | 'default'
}

const emptyValue = '__imagesilo_empty__'

function encodeValue(value: string) {
  return value === '' ? emptyValue : value
}

function decodeValue(value: string) {
  return value === emptyValue ? '' : value
}

export function Select({
  options,
  value,
  defaultValue = '',
  onValueChange,
  name,
  id,
  ariaLabel,
  placeholder,
  disabled = false,
  className,
  size = 'default',
}: SelectProps) {
  const controlled = value !== undefined
  const [internalValue, setInternalValue] = useState(defaultValue)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const selectedValue = controlled ? value : internalValue

  useEffect(() => {
    const form = triggerRef.current?.closest('form')
    if (!form || controlled) return
    const reset = () => setInternalValue(defaultValue)
    form.addEventListener('reset', reset)
    return () => form.removeEventListener('reset', reset)
  }, [controlled, defaultValue])

  function change(nextEncodedValue: string) {
    const nextValue = decodeValue(nextEncodedValue)
    if (!controlled) setInternalValue(nextValue)
    onValueChange?.(nextValue)
  }

  return (
    <div className={cn('ui-select', className)} data-size={size}>
      {name ? <input type="hidden" name={name} value={selectedValue} /> : null}
      <SelectPrimitive.Root value={encodeValue(selectedValue)} onValueChange={change} disabled={disabled}>
        <SelectPrimitive.Trigger ref={triggerRef} id={id} className="ui-select-trigger" aria-label={ariaLabel}>
          <SelectPrimitive.Value placeholder={placeholder} />
          <SelectPrimitive.Icon asChild><ChevronDown aria-hidden="true" /></SelectPrimitive.Icon>
        </SelectPrimitive.Trigger>
        <SelectPrimitive.Portal>
          <SelectPrimitive.Content className="ui-select-content" position="popper" sideOffset={6} collisionPadding={12}>
            <SelectPrimitive.ScrollUpButton className="ui-select-scroll"><ChevronUp /></SelectPrimitive.ScrollUpButton>
            <SelectPrimitive.Viewport className="ui-select-viewport">
              {options.map((option) => (
                <SelectPrimitive.Item className="ui-select-item" disabled={option.disabled} key={option.value || emptyValue} value={encodeValue(option.value)}>
                  <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                  <SelectPrimitive.ItemIndicator className="ui-select-check"><Check /></SelectPrimitive.ItemIndicator>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.Viewport>
            <SelectPrimitive.ScrollDownButton className="ui-select-scroll"><ChevronDown /></SelectPrimitive.ScrollDownButton>
          </SelectPrimitive.Content>
        </SelectPrimitive.Portal>
      </SelectPrimitive.Root>
    </div>
  )
}
