export const ROUTES = {
  home: "/",
  interviewIntro: "/interview",
  interviewRoom: "/interview/room",
  interviewReport: "/interview/report",
  interviewReportDetail: "/interview/report/detail",
  chat: "/chat",
  questionBank: "/question-bank",
  questionBankManage: "/question-bank/manage",
  auth: "/auth",
} as const;

export const CHAT_ROLES = {
  user: "user",
  assistant: "assistant",
} as const;

export type ChatRole = (typeof CHAT_ROLES)[keyof typeof CHAT_ROLES];

export const INTERVIEW_DEFAULTS = {
  initialMessageId: "1",
  assistantWelcomeMessage:
    "你好！我是你的 AI 棱镜。请先上传学习资料或笔记，我们开始今天的知识点讲解练习。",
  assistantFollowupMessage:
    "收到你的讲解。这是一个很好的切入点，但你能用自己的话再解释一下关键概念和适用场景吗？",
  aiReplyDelayMs: 1500,
  resumeAccept: ".pdf",
} as const;

export const MEDIA_TARGETS = {
  camera: "camera",
  microphone: "microphone",
} as const;

export type MediaTarget = (typeof MEDIA_TARGETS)[keyof typeof MEDIA_TARGETS];
