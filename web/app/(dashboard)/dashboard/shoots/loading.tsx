import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return <div className="space-y-4"><Skeleton className="h-8 w-40" /><Skeleton className="h-11 w-full max-w-sm" /><Skeleton className="h-24 w-full" /><Skeleton className="h-24 w-full" /></div>;
}
