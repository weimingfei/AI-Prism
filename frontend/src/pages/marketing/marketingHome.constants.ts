export const MARKETING_HERO_VIDEO_SOURCES = [
  "/videos/home-1.mp4",
  "/videos/home-2.mp4",
] as const;

export function selectMarketingHeroVideoSrc(now = Date.now()) {
  return MARKETING_HERO_VIDEO_SOURCES[now % MARKETING_HERO_VIDEO_SOURCES.length];
}

export const MARKETING_TEXT = {
  heroTitle: "灵知棱镜",
  heroSubtitle: "AI 讲解学习练习，帮你通过讲解、追问和复习真正掌握知识点",
  startNow: "立即体验",
  projectDemoTitle: "项目演示",
  projectDemoSubtitle:
    "这里先放一个演示占位视频，后续你可直接替换为正式项目演示视频。",
  advantagesTitle: "平台优势",
  workflowTitle: "使用流程",
  outcomeTitle: "你将获得",
} as const;

export const MARKETING_ADVANTAGES = [
  {
    icon: "target",
    title: "主题定制知识库",
    description: "按学习主题与资料内容生成高相关知识点，覆盖重点概念与应用场景。",
  },
  {
    icon: "brain",
    title: "实时追问与反馈",
    description: "沿用讲解学习追问逻辑，逐轮指出概念、表达与结构问题，给出改进建议。",
  },
  {
    icon: "resume",
    title: "资料与讲解联动分析",
    description: "从资料大纲到讲解表现做统一评分，帮你快速定位薄弱知识点。",
  },
  {
    icon: "report",
    title: "可复盘的学习报告",
    description: "自动沉淀评估记录与掌握度雷达，清晰看到每次练习的提升轨迹。",
  },
] as const;

export const MARKETING_WORKFLOW = [
  "上传资料并选择学习主题",
  "进入 AI 讲解练习页开始讲解问答",
  "根据反馈优化表达并继续练习",
  "生成学习报告并复盘薄弱点",
] as const;

export const MARKETING_OUTCOMES = [
  "清晰的知识掌握评分与改进方向",
  "可追溯的问答记录与复盘报告",
  "针对薄弱知识点的持续复习节奏",
] as const;
