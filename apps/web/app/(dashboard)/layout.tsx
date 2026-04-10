"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { MulticaIcon } from "@/components/multica-icon";
import { useNavigationStore } from "@/features/navigation";
import { SidebarProvider, SidebarInset, SidebarTrigger } from "@/components/ui/sidebar";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { AppSidebar } from "./_components/app-sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/");
    }
  }, [user, isLoading, router]);

  useEffect(() => {
    useNavigationStore.getState().onPathChange(pathname);
  }, [pathname]);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <MulticaIcon className="size-6" />
      </div>
    );
  }

  if (!user) return null;

  return (
    <SidebarProvider className="h-svh">
      <AppSidebar />
      <SidebarInset className="overflow-hidden">
        <div className="sticky top-0 z-20 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80 md:hidden">
          <div className="flex items-center gap-2 px-3 py-2">
            <SidebarTrigger className="-ml-1" />
            <div className="min-w-0">
              <div className="text-[10px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
                Workspace
              </div>
              <div className="truncate text-sm font-medium">
                {workspace?.name ?? "Loading"}
              </div>
            </div>
          </div>
        </div>
        {workspace ? (
          children
        ) : (
          <div className="flex flex-1 items-center justify-center">
            <MulticaIcon className="size-6 animate-pulse" />
          </div>
        )}
      </SidebarInset>
    </SidebarProvider>
  );
}
