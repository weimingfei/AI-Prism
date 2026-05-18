import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ROUTES } from "@/lib/constants";
import { buildReportSearch } from "@/lib/interviewReportRoute";
import { CHAT_MESSAGE_VARIANT } from "@/lib/chat";
import {
  buildInterviewProgressPatch,
  FEEDBACK_STREAM_DELAY_MS,
  FEEDBACK_STREAM_STEP,
  INTERVIEW_MESSAGE_GAP_MS,
  isInterviewResponseFailed,
  type InterviewFlowUser,
} from "@/hooks/interview/session/interviewSessionFlow.shared";
import { useInterviewAutoSave } from "@/hooks/interview/session/useInterviewAutoSave";
import { useInterviewMessageStream } from "@/hooks/interview/session/useInterviewMessageStream";
import { useInterviewProgressState } from "@/hooks/interview/session/useInterviewProgressState";
import { useInterviewRouteRecovery } from "@/hooks/interview/session/useInterviewRouteRecovery";
import { useInterviewSessionStorage } from "@/hooks/interview/session/useInterviewSessionStorage";
import { generateRequestId } from "@/hooks/interview/shared/interviewUtils";
import { interviewService } from "@/services/interviewService";

export function useInterviewSessionFlow(user: InterviewFlowUser) {
  const navigate = useNavigate();
  const params = useParams<{ sessionId?: string }>();
  const queryClient = useQueryClient();
  const [input, setInput] = useState("");
  const [isInterviewSubmitting, setIsInterviewSubmitting] = useState(false);
  const [interviewError, setInterviewError] = useState<string | null>(null);
  const [isEndingInterview, setIsEndingInterview] = useState(false);
  const [maxFollowUpCount, setMaxFollowUpCount] = useState(2);
  const [followUpMode, setFollowUpMode] = useState<"boundary" | "divergent">(
    "boundary",
  );

  const {
    interviewerSessionId: storedInterviewerSessionId,
    setInterviewerSessionId: persistInterviewerSessionId,
    clearStoredSession,
  } = useInterviewSessionStorage(user);
  const routeSessionId = params.sessionId?.trim() || null;
  const interviewerSessionId = routeSessionId || storedInterviewerSessionId;

  const {
    currentQuestionNumber,
    currentQuestionContent,
    isCurrentQuestionFollowUp,
    currentFollowUpCount,
    isInterviewFinished,
    totalInterviewScore,
    applyProgressPatch,
    resetProgressState,
  } = useInterviewProgressState();

  const {
    messages,
    appendAssistantMessage,
    appendNextQuestionMessage,
    appendSystemMessage,
    appendUserMessage,
    appendErrorMessage,
    startThinkingIndicator,
    stopThinkingIndicator,
    cancelActiveQuestionStream,
    resetMessageStream,
  } = useInterviewMessageStream();

  const isReady = Boolean(interviewerSessionId) && !isInterviewFinished;

  const buildInterviewRoomPath = useCallback(
    (sessionId: string) =>
      `${ROUTES.interviewRoom}/${encodeURIComponent(sessionId)}`,
    [],
  );

  const invalidateInterviewRecords = useCallback(
    () =>
      queryClient.invalidateQueries({
        queryKey: ["interview-records"],
      }),
    [queryClient],
  );

  const setInterviewerSessionId = useCallback(
    (nextValue: string | null) => {
      persistInterviewerSessionId(nextValue);
      if (nextValue) {
        navigate(buildInterviewRoomPath(nextValue), { replace: true });
        return;
      }
      navigate(ROUTES.interviewRoom, { replace: true });
    },
    [buildInterviewRoomPath, navigate, persistInterviewerSessionId],
  );

  const clearInterviewError = useCallback(() => {
    setInterviewError(null);
  }, []);

  const syncNextQuestion = useCallback(
    async (sessionId: string, options?: { appendMessage?: boolean }) => {
      const response = await interviewService.getCurrentQuestion(sessionId);
      if (isInterviewResponseFailed(response.isSuccess)) {
        throw new Error(
          response.errorMessage || "加载当前知识点失败，请重新上传资料后重试",
        );
      }

      const progressPatch = buildInterviewProgressPatch(response);
      applyProgressPatch(progressPatch);

      if (
        progressPatch.isInterviewFinished ||
        !progressPatch.currentQuestionContent
      ) {
        return;
      }

      await appendNextQuestionMessage(
        progressPatch.currentQuestionContent,
        progressPatch.currentQuestionNumber,
        progressPatch.isCurrentQuestionFollowUp,
        progressPatch.currentFollowUpCount,
        options,
      );
    },
    [appendNextQuestionMessage, applyProgressPatch],
  );

  useInterviewRouteRecovery({
    routeSessionId,
    storedInterviewerSessionId,
    interviewerSessionId,
    persistInterviewerSessionId,
    messages,
    syncNextQuestion,
    setInterviewError,
  });

  const { resetAutoSaveAttempt } = useInterviewAutoSave({
    interviewerSessionId,
    isInterviewFinished,
    appendSystemMessage,
    invalidateInterviewRecords,
  });

  const resetInterviewFlow = useCallback(() => {
    setInterviewerSessionId(null);
    resetProgressState();
    resetMessageStream();
    resetAutoSaveAttempt();
    setInterviewError(null);
    setInput("");
  }, [
    resetAutoSaveAttempt,
    resetMessageStream,
    resetProgressState,
    setInterviewerSessionId,
  ]);

  useEffect(() => {
    const handleLearningFinished = () => {
      applyProgressPatch({
        currentQuestionNumber: null,
        currentQuestionContent: null,
        isCurrentQuestionFollowUp: false,
        currentFollowUpCount: 0,
        isInterviewFinished: true,
      });
      appendSystemMessage(
        "所有知识点已处理完成，学习记录将自动保存。点击“结束学习”查看最终结果。",
      );
    };

    window.addEventListener(
      "ai-prism:learning-finished",
      handleLearningFinished,
    );
    return () => {
      window.removeEventListener(
        "ai-prism:learning-finished",
        handleLearningFinished,
      );
    };
  }, [appendSystemMessage, applyProgressPatch]);

  const pauseBetweenMessages = useCallback(async () => {
    await new Promise<void>((resolve) => {
      window.setTimeout(resolve, INTERVIEW_MESSAGE_GAP_MS);
    });
  }, []);

  const handleSend = useCallback(async () => {
    if (!isReady || isInterviewSubmitting) {
      return;
    }

    const nextInput = input.trim();
    if (!nextInput) {
      return;
    }
    const activeQuestionNumber = currentQuestionNumber?.trim();
    if (!activeQuestionNumber) {
      const message = "当前题号缺失，请先等待题目加载完成后再提交。";
      setInterviewError(message);
      appendErrorMessage(message);
      return;
    }

    setInterviewError(null);
    appendUserMessage(nextInput);
    setInput("");
    setIsInterviewSubmitting(true);
    startThinkingIndicator();

    try {
      const activeSessionId = interviewerSessionId;
      if (!activeSessionId) {
        throw new Error("请先上传并解析学习资料");
      }

      const response = await interviewService.answerInterviewQuestion({
        sessionId: activeSessionId,
        questionNumber: activeQuestionNumber,
        answerContent: nextInput,
        requestId: generateRequestId(),
        maxFollowUpCount,
        followUpMode,
      });
      stopThinkingIndicator();

      if (isInterviewResponseFailed(response.isSuccess)) {
        throw new Error(
          response.errorMessage || "提交讲解内容失败，请稍后重试",
        );
      }

      const progressPatch = buildInterviewProgressPatch(response);
      const feedbackText = response.feedback?.trim();
      applyProgressPatch(progressPatch);
      if (response.knowledgeList) {
        window.dispatchEvent(
          new CustomEvent("ai-prism:knowledge-list-updated", {
            detail: response.knowledgeList,
          }),
        );
      }

      if (feedbackText) {
        await appendAssistantMessage(feedbackText, {
          fakeStream: true,
          variant: CHAT_MESSAGE_VARIANT.feedback,
          streamStep: FEEDBACK_STREAM_STEP,
          streamDelayMs: FEEDBACK_STREAM_DELAY_MS,
        });
      }

      if (progressPatch.currentQuestionContent && !response.needsChoice) {
        if (feedbackText) {
          await pauseBetweenMessages();
        }
        await appendNextQuestionMessage(
          progressPatch.currentQuestionContent,
          progressPatch.currentQuestionNumber,
          progressPatch.isCurrentQuestionFollowUp,
          progressPatch.currentFollowUpCount,
        );
      }

      if (progressPatch.isInterviewFinished) {
        appendSystemMessage("学习已结束，正在保存记录...");
      }
    } catch (error) {
      stopThinkingIndicator();
      const message =
        error instanceof Error ? error.message : "提交失败，请稍后重试";
      setInterviewError(message);
      appendErrorMessage(message);
    } finally {
      setIsInterviewSubmitting(false);
    }
  }, [
    appendAssistantMessage,
    appendErrorMessage,
    appendNextQuestionMessage,
    appendSystemMessage,
    appendUserMessage,
    applyProgressPatch,
    currentQuestionNumber,
    followUpMode,
    input,
    interviewerSessionId,
    isInterviewSubmitting,
    isReady,
    maxFollowUpCount,
    pauseBetweenMessages,
    startThinkingIndicator,
    stopThinkingIndicator,
  ]);

  const handleFinishCurrentKnowledgePoint = useCallback(async () => {
    if (!interviewerSessionId || !currentQuestionNumber || isEndingInterview) {
      return;
    }
    try {
      const result =
        await interviewService.finishCurrentKnowledgePoint(
          interviewerSessionId,
        );
      if (result.knowledgeList) {
        window.dispatchEvent(
          new CustomEvent("ai-prism:knowledge-list-updated", {
            detail: result.knowledgeList,
          }),
        );
      }
      applyProgressPatch({
        currentQuestionNumber: null,
        currentQuestionContent: null,
        isCurrentQuestionFollowUp: false,
        currentFollowUpCount: 0,
        isInterviewFinished: Boolean(result.finished),
      });
      appendSystemMessage(
        result.finished
          ? "所有知识点已处理完成，学习记录将自动保存。点击“结束学习”查看最终结果。"
          : "当前知识点已结束，请从知识点清单选择下一个知识点继续练习。",
      );
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "结束当前知识点失败，请重试";
      setInterviewError(message);
      appendErrorMessage(message);
    }
  }, [
    appendErrorMessage,
    appendSystemMessage,
    applyProgressPatch,
    currentQuestionNumber,
    interviewerSessionId,
    isEndingInterview,
  ]);

  const handleEndInterview = useCallback(async () => {
    if (isEndingInterview) {
      return;
    }
    setIsEndingInterview(true);

    const reportSessionId = interviewerSessionId;
    try {
      if (reportSessionId) {
        await interviewService.finishInterviewSession(reportSessionId);
        await invalidateInterviewRecords();
      }
    } catch (error) {
      console.error("Save interview record failed:", error);
    } finally {
      stopThinkingIndicator();
      cancelActiveQuestionStream();
      persistInterviewerSessionId(null);
      clearStoredSession();
      resetProgressState();
      resetAutoSaveAttempt();
      navigate(
        `${ROUTES.interviewReport}${buildReportSearch(reportSessionId)}`,
        {
          state: reportSessionId ? { sessionId: reportSessionId } : undefined,
        },
      );
      setIsEndingInterview(false);
    }
  }, [
    cancelActiveQuestionStream,
    clearStoredSession,
    interviewerSessionId,
    invalidateInterviewRecords,
    isEndingInterview,
    navigate,
    persistInterviewerSessionId,
    resetAutoSaveAttempt,
    resetProgressState,
    stopThinkingIndicator,
  ]);

  return {
    messages,
    input,
    setInput,
    isReady,
    isInterviewSubmitting,
    interviewError,
    isEndingInterview,
    currentQuestionNumber,
    currentQuestionContent,
    isCurrentQuestionFollowUp,
    currentFollowUpCount,
    isInterviewFinished,
    totalInterviewScore,
    maxFollowUpCount,
    followUpMode,
    interviewerSessionId,
    setInterviewerSessionId,
    setMaxFollowUpCount,
    setFollowUpMode,
    handleFinishCurrentKnowledgePoint,
    clearInterviewError,
    resetInterviewFlow,
    syncNextQuestion,
    handleSend,
    handleEndInterview,
  };
}
