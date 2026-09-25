import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "ui-input h-10 w-full min-w-0 appearance-none rounded-[10px] border px-3.5 py-2 text-sm outline-none disabled:cursor-not-allowed",
        className
      )}
      {...props}
    />
  )
}

export { Input }
