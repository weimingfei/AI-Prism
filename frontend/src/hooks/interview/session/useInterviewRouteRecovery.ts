import { useEffect } from "react";
import { isMessageInWelcomeState } from "@/hooks/interview/session/interviewSessionFlow.shared";
import { AppError, ErrorCode, localizeErrorMessage } from "@/lib/errors";
import type { ChatMessage } from "@/lib/chat";

type UseInterviewRouteRecoveryParams = {
  routeSessionId: string | null;
  storedInterviewerSessionId: string | null;
  interviewerSessionId: string | null;
  persistInterviewerSessionId: (sessionId: string | null) => void;
  messages: ChatMessage[];
  syncNextQuestion: (
    sessionId: string,
    options?: { appendMessage?: boolean },
  ) => Promise<void>;
  setInterviewError: (message: string | null) => void;
};

export function useInterviewRouteRecovery({
  routeSessionId,
  storedInterviewerSessionId,
  interviewerSessionId,
  persistInterviewerSessionId,
  messages,
  syncNextQuestion,
  setInterviewError,
}: UseInterviewRouteRecoveryParams) {
  useEffect(() => {
    if (!routeSessionId) {
      return;
    }
    if (storedInterviewerSessionId === routeSessionId) {
      return;
    }
    persistInterviewerSessionId(routeSessionId);
  }, [persistInterviewerSessionId, routeSessionId, storedInterviewerSessionId]);

  useEffect(() => {
    if (!interviewerSessionId || !isMessageInWelcomeState(messages)) {
      return;
    }

    syncNextQuestion(interviewerSessionId).catch((error) => {
      setInterviewError(toFriendlyRestoreError(error));
    });
  }, [interviewerSessionId, messages, setInterviewError, syncNextQuestion]);
}

function toFriendlyRestoreError(error: unknown) {
  if (
    error instanceof AppError &&
    error.code === ErrorCode.RESOURCE_NOT_FOUND
  ) {
    return "当前学习会话已失效，请重新上传资料开始练习。";
  }
  if (error instanceof AppError) {
    return error.message || "恢复学习会话失败，请刷新页面后重试。";
  }
  if (error instanceof Error && error.message) {
    return error.message === "Requested resource not found" ||
      error.message === "请求的资源不存在。"
      ? "当前学习会话已失效，请重新上传资料开始练习。"
      : (localizeErrorMessage(
          error.message,
          "恢复学习会话失败，请刷新页面后重试。",
        ) ?? "恢复学习会话失败，请刷新页面后重试。");
  }
  return "恢复学习会话失败，请刷新页面后重试。";
}
