"use client"

import * as React from "react"
import { cn } from "cn"

function Command({ className, ...props }: React.ComponentProps<"div">) {
  return <div data-slot="command" className={cn("flex w-full flex-col overflow-hidden rounded-xl bg-popover text-popover-foreground", className)} {...props} />
}
function CommandInput({ className, ...props }: React.ComponentProps<"input">) {
  return <div className="border-b border-border p-2"><input data-slot="command-input" className={cn("flex h-10 w-full rounded-md bg-transparent px-2 text-sm outline-none placeholder:text-muted-foreground", className)} {...props} /></div>
}
function CommandList({ className, ...props }: React.ComponentProps<"div">) {
  return <div data-slot="command-list" className={cn("max-h-64 overflow-y-auto p-1", className)} {...props} />
}
function CommandEmpty({ className, ...props }: React.ComponentProps<"div">) {
  return <div data-slot="command-empty" className={cn("py-6 text-center text-sm text-muted-foreground", className)} {...props} />
}
function CommandItem({ className, ...props }: React.ComponentProps<"button">) {
  return <button type="button" data-slot="command-item" className={cn("flex min-h-11 w-full items-center rounded-md px-2 py-2 text-left text-sm hover:bg-muted focus-visible:bg-muted focus-visible:outline-none disabled:opacity-50", className)} {...props} />
}

export { Command, CommandEmpty, CommandInput, CommandItem, CommandList }
