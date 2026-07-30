import * as React from "react"

import { cn } from "@/lib/utils"

function Card({ className, size = "default", ...props }: React.ComponentProps<"div"> & { size?: "default" | "sm" }) {
  return (
    <div
      data-slot="card"
      data-size={size}
      className={cn(
        "group/card flex flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white text-gray-800 shadow-none dark:border-gray-800 dark:bg-white/[0.03] dark:text-white/90",
        className
      )}
      {...props}
    />
  )
}

export { Card }
