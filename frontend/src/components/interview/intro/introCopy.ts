export type IntroHighlight = {
  title: string;
  description: string;
};

export type InterviewIntroLocale = "zh-CN" | "en-US";

type InterviewIntroCopy = {
  badge: string;
  title: string;
  description: string;
  continueButton: string;
  startButton: string;
  reportButton: string;
  processTitle: string;
  processUpdateTitle: string;
  processUpdateDescription: string;
  sampleRadarTitle: string;
  highlights: IntroHighlight[];
  steps: string[];
  mockRadarPoints: Array<{
    label: string;
    value: number;
  }>;
};

const INTRO_COPY_BY_LOCALE: Record<InterviewIntroLocale, InterviewIntroCopy> = {
  "zh-CN": {
    badge: "AI 讲解学习评估",
    title: "用一条清晰、可控的流程检验你的知识掌握",
    description:
      "从资料分析开始，进入知识点讲解与追问，最后生成可追溯、可复习的学习报告。",
    continueButton: "继续上次学习",
    startButton: "进入练习室",
    reportButton: "查看示例报告",
    processTitle: "学习流程",
    processUpdateTitle: "知识库持续更新",
    processUpdateDescription:
      "覆盖重点概念、薄弱知识点与复习任务，持续提升掌握度。",
    sampleRadarTitle: "掌握度雷达图",
    highlights: [
      {
        title: "资料解析",
        description: "基于学习资料与笔记生成知识大纲",
      },
      {
        title: "讲解评分",
        description: "逻辑、结构、准确性与表达一致评估",
      },
      {
        title: "掌握雷达",
        description: "概念理解、表达清晰度、迁移应用综合画像",
      },
      {
        title: "复习计划",
        description: "根据掌握度生成后续复习节奏",
      },
    ],
    steps: [
      "上传资料与学习主题",
      "进入练习室，AI 实时追问",
      "生成知识卡片与复习建议",
    ],
    mockRadarPoints: [
      { label: "概念理解", value: 84 },
      { label: "讲解结构", value: 76 },
      { label: "表达清晰", value: 81 },
      { label: "追问应对", value: 72 },
      { label: "迁移应用", value: 88 },
    ],
  },
  "en-US": {
    badge: "AI Prism learning evaluation",
    title: "Check your knowledge mastery with a clean, guided flow",
    description:
      "Start from material analysis, continue with concept explanation and follow-up questions, and finish with a recoverable learning report.",
    continueButton: "Continue last session",
    startButton: "Enter practice room",
    reportButton: "View sample report",
    processTitle: "Learning flow",
    processUpdateTitle: "Knowledge base updates",
    processUpdateDescription:
      "Stay aligned with key concepts, weak spots, and review tasks.",
    sampleRadarTitle: "Mastery radar chart",
    highlights: [
      {
        title: "Material analysis",
        description: "Generate outlines from learning materials and notes",
      },
      {
        title: "Explanation score",
        description: "Evaluate logic, structure, accuracy, and expression together",
      },
      {
        title: "Mastery radar",
        description: "Concept understanding, clarity, transfer, and follow-up response",
      },
      {
        title: "Review plan",
        description: "Generate the next review rhythm from mastery level",
      },
    ],
    steps: [
      "Upload materials and choose a topic",
      "Explain concepts while AI asks follow-ups",
      "Receive knowledge cards and review suggestions",
    ],
    mockRadarPoints: [
      { label: "Concept mastery", value: 84 },
      { label: "Structure", value: 76 },
      { label: "Clarity", value: 81 },
      { label: "Follow-up response", value: 72 },
      { label: "Transfer", value: 88 },
    ],
  },
};

export const DEFAULT_INTERVIEW_INTRO_LOCALE: InterviewIntroLocale = "zh-CN";

export const getInterviewIntroCopy = (
  locale: InterviewIntroLocale = DEFAULT_INTERVIEW_INTRO_LOCALE,
) => {
  return INTRO_COPY_BY_LOCALE[locale] ?? INTRO_COPY_BY_LOCALE["zh-CN"];
};
