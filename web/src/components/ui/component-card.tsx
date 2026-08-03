import type * as React from "react"

import { cn } from "@/lib/utils"

type ComponentCardProps = React.ComponentProps<"section"> & {
  title: React.ReactNode
  description?: React.ReactNode
  bodyClassName?: string
}

function ComponentCard({
  title,
  description,
  className,
  bodyClassName,
  children,
  ...props
}: ComponentCardProps) {
  return (
    <section
      className={cn(
        "min-w-0 rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-card",
        className
      )}
      {...props}
    >
      <div className="px-6 py-5">
        <h2 className="text-base font-medium text-gray-800 dark:text-white/90">{title}</h2>
        {description ? <p className="mt-1 text-xs leading-[18px] text-gray-500 dark:text-gray-400">{description}</p> : null}
      </div>
      <div className={cn("border-t border-gray-100 p-4 dark:border-gray-800 sm:p-6", bodyClassName)}>
        {children}
      </div>
    </section>
  )
}

export { ComponentCard }
