"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { CheckSquare, CalendarPlus, MessageCircle, Plane } from "lucide-react";
import { cn } from "@/lib/utils";

const links = [
  { href: "/tasks", label: "Tasks", icon: CheckSquare },
  { href: "/chat", label: "Chat", icon: MessageCircle },
  { href: "/events", label: "Event", icon: CalendarPlus },
  { href: "/", label: "Visa", icon: Plane },
] as const;

export function BottomNav() {
  const pathname = usePathname();

  return (
    <nav className="fixed bottom-0 inset-x-0 z-50 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="mx-auto flex h-16 max-w-md items-center justify-around px-4">
        {links.map(({ href, label, icon: Icon }) => {
          const active = pathname === href;
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex flex-col items-center gap-1 text-xs transition-colors",
                active
                  ? "text-primary font-medium"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              <Icon className="h-5 w-5" />
              {label}
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
