import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg border border-transparent text-sm font-medium whitespace-nowrap transition-all outline-none select-none focus-visible:ring-4 focus-visible:ring-brand-500/12 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "bg-brand-500 text-white shadow-theme-xs hover:bg-brand-600",
        outline:
          "border-gray-300 bg-white text-gray-700 shadow-theme-xs hover:bg-gray-50 hover:text-gray-800 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:border-gray-600 dark:hover:bg-gray-700 dark:hover:text-white",
        secondary:
          "border-gray-200 bg-gray-100 text-gray-700 hover:bg-gray-200 dark:border-gray-800 dark:bg-white/[0.03] dark:text-gray-300 dark:hover:bg-white/[0.06]",
        ghost:
          "text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-white",
        destructive:
          "bg-error-500 text-white shadow-theme-xs hover:bg-error-600 focus-visible:ring-error-500/15",
        link: "h-auto border-0 p-0 text-brand-500 shadow-none hover:text-brand-600 hover:underline",
      },
      size: {
        default: "h-10 px-4",
        xs: "h-8 px-2.5",
        sm: "h-9 px-3.5",
        lg: "h-11 px-5",
        icon: "size-10 rounded-full px-0",
        "icon-xs":
          "size-8 px-0 [&_svg:not([class*='size-'])]:size-4",
        "icon-sm":
          "size-9 rounded-full px-0",
        "icon-lg": "size-12 rounded-full px-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  children,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const classes = cn(buttonVariants({ variant, size, className }))

  if (asChild && React.isValidElement(children)) {
    const child = children as React.ReactElement<Record<string, unknown>>
    return React.cloneElement(child, {
      ...props,
      "data-slot": "button",
      "data-variant": variant,
      "data-size": size,
      className: cn(classes, typeof child.props.className === "string" ? child.props.className : undefined),
    })
  }

  return (
    <button
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={classes}
      {...props}
    >
      {children}
    </button>
  )
}

export { Button }
