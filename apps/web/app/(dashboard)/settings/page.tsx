"use client";

import { useMemo } from "react";
import {
  User,
  Palette,
  Key,
  Settings,
  Users,
  FolderGit2,
  Lightbulb,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { useWorkspaceStore } from "@/features/workspace";
import { useIsMobile } from "@/hooks/use-mobile";
import { useQueryParamSelection } from "@/shared/hooks/use-query-param-selection";
import { AccountTab } from "./_components/account-tab";
import { AppearanceTab } from "./_components/general-tab";
import { TokensTab } from "./_components/tokens-tab";
import { WorkspaceTab } from "./_components/workspace-tab";
import { MembersTab } from "./_components/members-tab";
import { RepositoriesTab } from "./_components/repositories-tab";
import { IdeaOSTab } from "./_components/ideaos-tab";

const accountTabs = [
  { value: "profile", label: "Profile", icon: User },
  { value: "appearance", label: "Appearance", icon: Palette },
  { value: "tokens", label: "API Tokens", icon: Key },
];

const workspaceTabs = [
  { value: "workspace", label: "General", icon: Settings },
  { value: "repositories", label: "Repositories", icon: FolderGit2 },
  { value: "ideas", label: "Ideas", icon: Lightbulb },
  { value: "members", label: "Members", icon: Users },
];

const allTabs = [...accountTabs, ...workspaceTabs];

function SettingsContent() {
  return (
    <>
      <TabsContent value="profile"><AccountTab /></TabsContent>
      <TabsContent value="appearance"><AppearanceTab /></TabsContent>
      <TabsContent value="tokens"><TokensTab /></TabsContent>
      <TabsContent value="workspace"><WorkspaceTab /></TabsContent>
      <TabsContent value="repositories"><RepositoriesTab /></TabsContent>
      <TabsContent value="ideas"><IdeaOSTab /></TabsContent>
      <TabsContent value="members"><MembersTab /></TabsContent>
    </>
  );
}

function MobileSettingsList({
  workspaceName,
  onSelect,
}: {
  workspaceName?: string;
  onSelect: (value: string) => void;
}) {
  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <div className="border-b px-4 py-3">
        <h1 className="text-base font-semibold">Settings</h1>
      </div>
      <div className="flex-1 overflow-y-auto px-3 py-4">
        <div>
          <div className="px-3 pb-2 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
            My Account
          </div>
          <div className="space-y-1">
            {accountTabs.map((tab) => (
              <button
                key={tab.value}
                type="button"
                onClick={() => onSelect(tab.value)}
                className="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-left transition-colors hover:bg-muted"
              >
                <tab.icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate text-sm font-medium">
                  {tab.label}
                </span>
                <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
              </button>
            ))}
          </div>
        </div>

        <div className="mt-6">
          <div className="px-3 pb-2 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
            {workspaceName ?? "Workspace"}
          </div>
          <div className="space-y-1">
            {workspaceTabs.map((tab) => (
              <button
                key={tab.value}
                type="button"
                onClick={() => onSelect(tab.value)}
                className="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-left transition-colors hover:bg-muted"
              >
                <tab.icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate text-sm font-medium">
                  {tab.label}
                </span>
                <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export default function SettingsPage() {
  const workspaceName = useWorkspaceStore((s) => s.workspace?.name);
  const isMobile = useIsMobile();
  const [urlTab, setUrlTab] = useQueryParamSelection("tab");

  const validTabs = useMemo(
    () => new Map(allTabs.map((item) => [item.value, item])),
    [],
  );

  const activeTab = urlTab && validTabs.has(urlTab)
    ? urlTab
    : isMobile
      ? ""
      : "profile";

  const activeTabMeta = activeTab ? validTabs.get(activeTab) ?? null : null;

  if (isMobile) {
    if (!activeTab) {
      return (
        <MobileSettingsList
          workspaceName={workspaceName}
          onSelect={(value) => setUrlTab(value)}
        />
      );
    }

    return (
      <Tabs value={activeTab} className="flex-1 min-h-0 gap-0">
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex h-12 shrink-0 items-center gap-2 border-b px-3">
            <Button
              variant="ghost"
              size="sm"
              className="-ml-2"
              onClick={() => setUrlTab("")}
            >
              <ChevronLeft className="h-4 w-4" />
              Settings
            </Button>
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">
                {activeTabMeta?.label ?? "Settings"}
              </div>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto">
            <div className="mx-auto w-full max-w-3xl px-4 py-4">
              <SettingsContent />
            </div>
          </div>
        </div>
      </Tabs>
    );
  }

  return (
    <Tabs
      value={activeTab}
      onValueChange={(value) => setUrlTab(value)}
      orientation="vertical"
      className="flex-1 min-h-0 gap-0"
    >
      <div className="w-52 shrink-0 border-r overflow-y-auto p-4">
        <h1 className="mb-4 px-2 text-sm font-semibold">Settings</h1>
        <TabsList variant="line" className="flex-col items-stretch">
          <span className="px-2 pb-1 pt-2 text-xs font-medium text-muted-foreground">
            My Account
          </span>
          {accountTabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </TabsTrigger>
          ))}

          <span className="truncate px-2 pb-1 pt-4 text-xs font-medium text-muted-foreground">
            {workspaceName ?? "Workspace"}
          </span>
          {workspaceTabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </div>

      <div className="flex-1 min-w-0 overflow-y-auto">
        <div className="mx-auto w-full max-w-3xl p-6">
          <SettingsContent />
        </div>
      </div>
    </Tabs>
  );
}
