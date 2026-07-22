import type { ButtonHTMLAttributes, ReactNode } from "react";
import { buttonVariants, type ButtonVariant } from "./styles";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  children?: ReactNode;
}

export default function Button({ variant = "primary", className = "", children, ...props }: ButtonProps) {
  return (
    <button
      {...props}
      className={
        "inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-medium " +
        "transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 " +
        "disabled:cursor-not-allowed disabled:opacity-50 " +
        buttonVariants[variant] +
        " " +
        className
      }
    >
      {children}
    </button>
  );
}
