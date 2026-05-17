import { useEffect, useMemo, useState } from "react";
import { Bot, Check, KeyRound, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  AI_PROVIDER_METAS,
  createDefaultAiProviderSettings,
  loadAiProviderSettings,
  saveAiProviderSettings,
  type AiProviderConfig,
  type AiProviderId,
  type AiProviderSettings,
} from "@/lib/aiProviderSettings";
import { cn } from "@/lib/utils";

type AiProviderSettingsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

const providerIconClass = (providerId: AiProviderId) => {
  switch (providerId) {
    case "deepseek":
      return "text-blue-600";
    case "doubao":
      return "text-violet-600";
    case "openai":
      return "text-slate-900";
    case "gemini":
      return "text-amber-500";
  }
};

export default function AiProviderSettingsDialog({
  open,
  onOpenChange,
}: AiProviderSettingsDialogProps) {
  const [settings, setSettings] = useState<AiProviderSettings>(() =>
    loadAiProviderSettings(),
  );

  useEffect(() => {
    if (open) {
      queueMicrotask(() => setSettings(loadAiProviderSettings()));
    }
  }, [open]);

  const activeMeta = useMemo(
    () =>
      AI_PROVIDER_METAS.find((item) => item.id === settings.activeProviderId) ??
      AI_PROVIDER_METAS[0],
    [settings.activeProviderId],
  );
  const activeConfig = settings.providers[activeMeta.id];

  const updateActiveProvider = (patch: Partial<AiProviderConfig>) => {
    setSettings((prev) => ({
      ...prev,
      providers: {
        ...prev.providers,
        [prev.activeProviderId]: {
          ...prev.providers[prev.activeProviderId],
          ...patch,
        },
      },
    }));
  };

  const handleSave = () => {
    saveAiProviderSettings(settings);
    onOpenChange(false);
  };

  const handleReset = () => {
    setSettings(createDefaultAiProviderSettings());
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-5xl gap-0 p-0">
        <div className="grid min-h-[620px] grid-cols-[280px_1fr]">
          <aside className="border-r bg-slate-50 p-5">
            <DialogHeader className="mb-6 text-left">
              <DialogTitle className="flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-indigo-600" />
                AI服务商
              </DialogTitle>
              <DialogDescription>
                留空时默认使用 application.toml 中配置的本地大模型。
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              {AI_PROVIDER_METAS.map((provider) => {
                const config = settings.providers[provider.id];
                const configured = config.apiKey.trim() !== "";
                const active = provider.id === settings.activeProviderId;
                return (
                  <button
                    key={provider.id}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-lg border px-3 py-3 text-left transition-colors",
                      active
                        ? "border-slate-900 bg-white"
                        : "border-transparent hover:border-slate-200 hover:bg-white",
                    )}
                    onClick={() =>
                      setSettings((prev) => ({
                        ...prev,
                        activeProviderId: provider.id,
                      }))
                    }
                  >
                    <Bot className={cn("h-5 w-5", providerIconClass(provider.id))} />
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-medium text-slate-900">
                        {provider.name}
                      </span>
                      <span className="block text-xs text-slate-500">
                        {configured ? "已配置" : "未配置"}
                      </span>
                    </span>
                    <span
                      className={cn(
                        "flex h-7 w-7 items-center justify-center rounded-full border",
                        active
                          ? "border-slate-900 bg-slate-900 text-white"
                          : "border-slate-300 bg-white text-transparent",
                      )}
                    >
                      <Check className="h-4 w-4" />
                    </span>
                  </button>
                );
              })}
            </div>
          </aside>

          <main className="p-8">
            <div className="mb-8 flex items-start gap-4">
              <Bot className={cn("mt-1 h-8 w-8", providerIconClass(activeMeta.id))} />
              <div>
                <h3 className="text-2xl font-semibold text-slate-950">
                  {activeMeta.name}
                </h3>
                <p className="mt-2 text-sm text-slate-500">
                  {activeMeta.description}
                </p>
              </div>
            </div>

            <div className="max-w-3xl space-y-6">
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label htmlFor="ai-provider-api-key">API Key</Label>
                  <span className="flex items-center gap-1 text-xs text-slate-500">
                    <KeyRound className="h-3.5 w-3.5" />
                    仅保存在当前浏览器
                  </span>
                </div>
                <Input
                  id="ai-provider-api-key"
                  type="password"
                  value={activeConfig.apiKey}
                  placeholder="API Key"
                  onChange={(event) =>
                    updateActiveProvider({ apiKey: event.target.value })
                  }
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="ai-provider-model-id">模型 ID</Label>
                <Input
                  id="ai-provider-model-id"
                  value={activeConfig.modelId}
                  placeholder={activeMeta.defaultModelId || "模型 ID"}
                  onChange={(event) =>
                    updateActiveProvider({ modelId: event.target.value })
                  }
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="ai-provider-endpoint">
                  API 端点，如：https://openai.example.org/v1
                </Label>
                <Input
                  id="ai-provider-endpoint"
                  value={activeConfig.endpoint}
                  placeholder={activeMeta.defaultEndpoint}
                  disabled={!activeMeta.endpointEditable}
                  onChange={(event) =>
                    updateActiveProvider({ endpoint: event.target.value })
                  }
                />
              </div>
            </div>
          </main>
        </div>
        <DialogFooter className="border-t px-6 py-4">
          <Button variant="outline" onClick={handleReset}>
            清空配置
          </Button>
          <Button onClick={handleSave}>保存</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
