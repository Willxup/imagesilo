import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center gap-1.5 rounded-[10px] border border-transparent text-sm font-medium whitespace-nowrap outline-none select-none focus-visible:ring-4 focus-visible:ring-brand-500/12 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "[background:var(--silo-gradient)] text-white hover:opacity-90",
        brandSolid: "[background-color:var(--accent)] [color:var(--accent-foreground)] hover:opacity-90",
        outline:
          "[border-color:var(--tail-border)] [background-color:var(--tail-surface)] [color:var(--tail-text)] hover:[background-color:var(--tail-surface-soft)]",
        secondary:
          "[background-color:var(--tail-surface-soft)] [color:var(--tail-text)] hover:[background-color:var(--tail-brand-soft)]",
        ghost:
          "[color:var(--tail-text-soft)] hover:[background-color:var(--tail-surface-soft)] hover:[color:var(--tail-text)]",
        destructive:
          "[background-color:var(--color-danger)] text-white hover:opacity-90 focus-visible:ring-error-500/15",
        link: "h-auto border-0 p-0 [color:var(--accent)] shadow-none hover:underline",
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
