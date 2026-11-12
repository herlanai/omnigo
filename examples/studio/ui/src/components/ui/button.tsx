import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
  { variants: {
    variant: { default: "bg-primary text-primary-foreground hover:opacity-90", outline: "border border-input bg-background hover:bg-muted" },
    size: { default: "h-10 px-4", sm: "h-8 px-3", icon: "size-10" },
  }, defaultVariants: { variant: "default", size: "default" } },
);
function Button({ className, variant, size, asChild = false, type, ...props }: ComponentProps<"button"> & VariantProps<typeof buttonVariants> & { asChild?: boolean }) {
  const Component = asChild ? Slot : "button";
  return <Component data-slot="button" type={asChild ? undefined : type ?? "button"} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
export { Button, buttonVariants };
