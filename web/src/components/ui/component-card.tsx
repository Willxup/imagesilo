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
        "component-card min-w-0 rounded-2xl border",
        className
      )}
      {...props}
    >
      <div className="component-card-heading">
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
      </div>
      <div className={cn("component-card-body", bodyClassName)}>
        {children}
      </div>
    </section>
  )
}

export { ComponentCard }
