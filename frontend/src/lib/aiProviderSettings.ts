export type AiProviderId = "deepseek" | "doubao" | "openai" | "gemini";

export type AiProviderConfig = {
  providerId: AiProviderId;
  apiKey: string;
  modelId: string;
  endpoint: string;
};

export type AiProviderMeta = {
  id: AiProviderId;
  name: string;
  description: string;
  defaultEndpoint: string;
  defaultModelId: string;
  endpointEditable: boolean;
};

const STORAGE_KEY = "lingzhi:ai-provider-settings:v1";

export const AI_PROVIDER_METAS: AiProviderMeta[] = [
  {
    id: "deepseek",
    name: "DeepSeek",
    description: "在 DeepSeek 开放平台获取 API 密钥",
    defaultEndpoint: "https://api.deepseek.com/v1",
    defaultModelId: "deepseek-chat",
    endpointEditable: false,
  },
  {
    id: "doubao",
    name: "豆包",
    description: "在火山引擎获取 API 密钥",
    defaultEndpoint: "https://ark.cn-beijing.volces.com/api/v3",
    defaultModelId: "",
    endpointEditable: false,
  },
  {
    id: "openai",
    name: "OpenAI",
    description: "在 OpenAI 或兼容 OpenAI 格式的开放平台获取 API 密钥",
    defaultEndpoint: "https://api.openai.com/v1",
    defaultModelId: "",
    endpointEditable: true,
  },
  {
    id: "gemini",
    name: "Gemini",
    description: "支持润色、语法检查和 PDF 图片识别导入。推荐模型 ID：gemini-flash-latest",
    defaultEndpoint: "https://generativelanguage.googleapis.com/v1beta/openai",
    defaultModelId: "gemini-flash-latest",
    endpointEditable: false,
  },
];

const createEmptyConfig = (providerId: AiProviderId): AiProviderConfig => {
  const meta = AI_PROVIDER_METAS.find((item) => item.id === providerId)!;
  return {
    providerId,
    apiKey: "",
    modelId: meta.defaultModelId,
    endpoint: meta.defaultEndpoint,
  };
};

export type AiProviderSettings = {
  activeProviderId: AiProviderId;
  providers: Record<AiProviderId, AiProviderConfig>;
};

export const createDefaultAiProviderSettings = (): AiProviderSettings => ({
  activeProviderId: "doubao",
  providers: {
    deepseek: createEmptyConfig("deepseek"),
    doubao: createEmptyConfig("doubao"),
    openai: createEmptyConfig("openai"),
    gemini: createEmptyConfig("gemini"),
  },
});

export const loadAiProviderSettings = (): AiProviderSettings => {
  if (typeof window === "undefined") {
    return createDefaultAiProviderSettings();
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return createDefaultAiProviderSettings();
    }
    const parsed = JSON.parse(raw) as Partial<AiProviderSettings>;
    const defaults = createDefaultAiProviderSettings();
    return {
      activeProviderId:
        parsed.activeProviderId && parsed.activeProviderId in defaults.providers
          ? parsed.activeProviderId
          : defaults.activeProviderId,
      providers: {
        ...defaults.providers,
        ...(parsed.providers ?? {}),
      },
    };
  } catch {
    return createDefaultAiProviderSettings();
  }
};

export const saveAiProviderSettings = (settings: AiProviderSettings) => {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  window.dispatchEvent(new CustomEvent("lingzhi:ai-provider-settings-updated"));
};

export const resolveActiveAiProviderHeaders = () => {
  const settings = loadAiProviderSettings();
  const active = settings.providers[settings.activeProviderId];
  const apiKey = active.apiKey.trim();
  const modelId = active.modelId.trim();
  const endpoint = active.endpoint.trim();

  if (!apiKey) {
    return null;
  }

  return {
    "X-Lingzhi-AI-Provider": active.providerId === "gemini" ? "openai" : "openai",
    "X-Lingzhi-AI-Base-URL": endpoint,
    "X-Lingzhi-AI-API-Key": apiKey,
    "X-Lingzhi-AI-Model": modelId,
  };
};
