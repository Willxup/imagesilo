import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-10 w-full min-w-0 appearance-none rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-sm text-gray-800 shadow-theme-xs transition-colors outline-none file:mr-4 file:border-0 file:bg-gray-50 file:px-3 file:py-2 file:text-sm file:font-medium file:text-gray-700 placeholder:text-gray-400 focus:border-brand-300 focus:ring-4 focus:ring-brand-500/10 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:opacity-50 aria-invalid:border-error-500 aria-invalid:ring-error-500/15 dark:border-gray-800 dark:bg-gray-900 dark:text-white/90 dark:placeholder:text-gray-500 dark:focus:border-brand-500 dark:disabled:bg-gray-950",
        className
      )}
      {...props}
    />
  )
}

export { Input }
