import { Check } from 'lucide-react'
import type { ComponentProps } from 'react'

import { cn } from '../../lib/utils'

export function Checkbox({ className, ...props }: ComponentProps<'input'>) {
  return (
    <span className={cn('ui-checkbox', className)}>
      <input type="checkbox" {...props} />
      <Check aria-hidden="true" className="ui-checkbox-check" strokeWidth={2.4} />
    </span>
  )
}
