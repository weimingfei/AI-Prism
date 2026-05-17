import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from "react";
import {
  createInitialAudioTranscriptionState,
  getMergedAudioTranscription,
  reduceAudioTranscriptionState,
} from "@/lib/audioTranscription";
import { useAudioTranscriptionTransport } from "@/hooks/audio/useAudioTranscriptionTransport";
import { useMicrophonePcmStream } from "@/hooks/audio/useMicrophonePcmStream";
import type { UserRespDTO } from "@/types/auth";

const AUDIO_SAMPLE_RATE = 16000;
const START_RECORDING_ERROR =
  "无法访问麦克风或连接语音转写服务";
const MICROPHONE_PERMISSION_ERROR =
  "麦克风权限被系统或浏览器拒绝。请在 Chrome 地址栏左侧权限图标和 Windows 设置中允许麦克风访问。";

const resolveAudioUserId = (currentUser: UserRespDTO | null) => {
  const normalizedUsername = currentUser?.username?.trim();
  const normalizedUserId =
    typeof currentUser?.id === "number" && currentUser.id > 0
      ? String(currentUser.id)
      : null;

  return normalizedUsername || normalizedUserId || null;
};

export function useAudioTranscriptionController(
  currentUser: UserRespDTO | null,
) {
  const [isRecording, setIsRecording] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [transcriptionState, dispatchTranscription] = useReducer(
    reduceAudioTranscriptionState,
    undefined,
    createInitialAudioTranscriptionState,
  );
  const cleanupPromiseRef = useRef<Promise<void> | null>(null);
  const cleanupRef = useRef<() => Promise<void>>(async () => undefined);
  const activeStartTokenRef = useRef<symbol | null>(null);

  const {
    connect: connectTransport,
    disconnect: disconnectTransport,
    sendAudioChunk,
  } = useAudioTranscriptionTransport({
    userId: resolveAudioUserId(currentUser),
    onReplace: useCallback((text: string) => {
      dispatchTranscription({
        kind: "replace",
        text,
      });
    }, []),
    onArchive: useCallback((text: string) => {
      dispatchTranscription({
        kind: "archive",
        text,
      });
    }, []),
    onError: useCallback((message: string) => {
      setError(message);
      void cleanupRef.current();
    }, []),
  });

  const stream = useMicrophonePcmStream({
    sampleRate: AUDIO_SAMPLE_RATE,
    onChunk: sendAudioChunk,
    onError: useCallback((streamError: unknown) => {
      console.error("Microphone PCM stream failed:", streamError);
      setError(START_RECORDING_ERROR);
      void cleanupRef.current();
    }, []),
  });

  const { start: startStream, stop: stopStream } = stream;

  const resolveStartErrorMessage = useCallback((error: unknown) => {
    if (error instanceof DOMException) {
      if (error.name === "NotAllowedError" || error.name === "SecurityError") {
        return MICROPHONE_PERMISSION_ERROR;
      }
      if (error.name === "NotFoundError") {
        return "没有检测到可用麦克风，请检查设备连接后重试。";
      }
      if (error.name === "NotReadableError") {
        return "麦克风正被其他应用占用，请关闭占用麦克风的软件后重试。";
      }
    }

    if (error instanceof Error) {
      const message = error.message.toLowerCase();
      if (message.includes("permission denied")) {
        return MICROPHONE_PERMISSION_ERROR;
      }
    }

    return START_RECORDING_ERROR;
  }, []);

  const cleanup = useCallback(async () => {
    if (cleanupPromiseRef.current) {
      await cleanupPromiseRef.current;
      return;
    }

    cleanupPromiseRef.current = (async () => {
      activeStartTokenRef.current = null;
      disconnectTransport();
      await stopStream();
      setIsRecording(false);
    })();

    try {
      await cleanupPromiseRef.current;
    } finally {
      cleanupPromiseRef.current = null;
    }
  }, [disconnectTransport, stopStream]);

  useEffect(() => {
    cleanupRef.current = cleanup;
  }, [cleanup]);

  const startRecording = useCallback(async () => {
    if (!currentUser) {
      setError("User is not logged in");
      return;
    }

    if (isRecording) {
      return;
    }

    try {
      const startToken = Symbol("audio-transcription-start");
      activeStartTokenRef.current = startToken;
      setError(null);
      dispatchTranscription({
        kind: "reset",
      });
      await startStream();
      if (activeStartTokenRef.current !== startToken) {
        return;
      }
      connectTransport();
      setIsRecording(true);
    } catch (startError) {
      console.error("Start recording failed:", startError);
      setError(resolveStartErrorMessage(startError));
      await cleanup();
    }
  }, [
    cleanup,
    connectTransport,
    currentUser,
    isRecording,
    resolveStartErrorMessage,
    startStream,
  ]);

  const stopRecording = useCallback(() => {
    void cleanup();
  }, [cleanup]);

  useEffect(() => {
    return () => {
      void cleanup();
    };
  }, [cleanup]);

  return useMemo(
    () => ({
      isRecording,
      currentSentence: transcriptionState.liveText,
      historySentences: transcriptionState.finalText
        ? transcriptionState.finalText
            .split(/\n\n+/)
            .map((sentence) => sentence.trim())
            .filter(Boolean)
        : [],
      transcription: getMergedAudioTranscription(transcriptionState),
      error,
      startRecording,
      stopRecording,
    }),
    [error, isRecording, startRecording, stopRecording, transcriptionState],
  );
}
