import * as React from "react"

import { cn } from "@/lib/utils"

function Card({ className, size = "default", ...props }: React.ComponentProps<"div"> & { size?: "default" | "sm" }) {
  return (
    <div
      data-slot="card"
      data-size={size}
      className={cn(
        "group/card flex flex-col overflow-hidden rounded-2xl border [border-color:var(--tail-border)] [background-color:var(--tail-card)] [color:var(--tail-text)] shadow-none",
        className
      )}
      {...props}
    />
  )
}

export { Card }
