"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { cn } from "cn";

export function FadeIn({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reduceMotion) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { threshold: 0.15 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={ref}
      className={cn(
        "transition-[opacity,transform] duration-500 ease-out motion-reduce:transition-none motion-reduce:transform-none motion-reduce:opacity-100 motion-reduce:translate-y-0",
        visible ? "opacity-100 translate-y-0" : "opacity-0 translate-y-3",
        className,
      )}
    >
      {children}
    </div>
  );
}
