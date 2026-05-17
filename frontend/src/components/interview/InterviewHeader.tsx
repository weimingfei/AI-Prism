import { NotebookPen, Video, VideoOff } from "lucide-react";
import { Button } from "@/components/ui/button";

type InterviewHeaderProps = {
  isReady: boolean;
  currentQuestionNumber: string | null;
  isCurrentQuestionFollowUp: boolean;
  currentFollowUpCount: number;
  isInterviewFinished: boolean;
  totalInterviewScore: number | null;
  maxFollowUpCount: number;
  followUpMode: "boundary" | "divergent";
  isCameraOpen: boolean;
  isEndingInterview: boolean;
  onMaxFollowUpCountChange: (count: number) => void;
  onFollowUpModeChange: (mode: "boundary" | "divergent") => void;
  onToggleCamera: () => void;
  onOpenSketchpad: () => void;
  onFinishCurrentKnowledgePoint: () => void;
  onEndInterview: () => void;
};

export default function InterviewHeader({
  isReady,
  currentQuestionNumber,
  isCurrentQuestionFollowUp,
  currentFollowUpCount,
  isInterviewFinished,
  totalInterviewScore,
  maxFollowUpCount,
  followUpMode,
  isCameraOpen,
  isEndingInterview,
  onMaxFollowUpCountChange,
  onFollowUpModeChange,
  onToggleCamera,
  onOpenSketchpad,
  onFinishCurrentKnowledgePoint,
  onEndInterview,
}: InterviewHeaderProps) {
  return (
    <div className="sticky top-0 z-10 flex items-center justify-between border-b bg-white/70 px-6 py-4 backdrop-blur-sm">
      <div>
        <h2 className="text-lg font-semibold text-slate-900">
          AI 棱镜
        </h2>
        <p className="text-sm text-slate-500">实时知识点讲解练习</p>
      </div>
      <div className="flex items-center gap-3">
        <div className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-500">
          {isReady ? "练习进行中" : "待上传资料"}
        </div>
        {currentQuestionNumber && !isInterviewFinished ? (
          <div className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-500">
            当前知识点：{currentQuestionNumber}
          </div>
        ) : null}
        {isCurrentQuestionFollowUp && !isInterviewFinished ? (
          <div className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs text-amber-700">
            第 {currentFollowUpCount} 次追问
          </div>
        ) : null}
        {totalInterviewScore !== null ? (
          <div className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-500">
            当前掌握度：{totalInterviewScore}
          </div>
        ) : null}
        <label className="flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-500">
          追问
          <select
            className="bg-transparent text-xs outline-none"
            value={maxFollowUpCount}
            disabled={isInterviewFinished || isEndingInterview}
            onChange={(event) =>
              onMaxFollowUpCountChange(Number(event.target.value))
            }
          >
            {[0, 1, 2, 3, 4, 5].map((count) => (
              <option key={count} value={count}>
                {count}
              </option>
            ))}
          </select>
          次
        </label>
        <label className="flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs text-slate-500">
          模式
          <select
            className="bg-transparent text-xs outline-none"
            value={followUpMode}
            disabled={isInterviewFinished || isEndingInterview}
            onChange={(event) =>
              onFollowUpModeChange(
                event.target.value === "divergent" ? "divergent" : "boundary",
              )
            }
          >
            <option value="boundary">边界模式</option>
            <option value="divergent">发散模式</option>
          </select>
        </label>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={onOpenSketchpad}>
            <NotebookPen className="mr-2 h-4 w-4" />
            构思板
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!currentQuestionNumber || isInterviewFinished || isEndingInterview}
            onClick={onFinishCurrentKnowledgePoint}
          >
            结束当前知识点
          </Button>
          <Button variant="outline" size="sm" onClick={onToggleCamera}>
            {isCameraOpen ? (
              <Video className="mr-2 h-4 w-4" />
            ) : (
              <VideoOff className="mr-2 h-4 w-4" />
            )}
            {isCameraOpen ? "关闭摄像头" : "开启摄像头"}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            disabled={isEndingInterview}
            onClick={onEndInterview}
          >
            {isEndingInterview ? "处理中..." : "结束学习"}
          </Button>
        </div>
      </div>
    </div>
  );
}
