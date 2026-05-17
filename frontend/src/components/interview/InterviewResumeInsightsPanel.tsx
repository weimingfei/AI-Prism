import ErrorNotice from "@/components/feedback/ErrorNotice";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Check, X } from "lucide-react";
import type { KnowledgePointDTO } from "@/services/interviewService";

type InterviewResumeInsightsPanelProps = {
  resumeScore: number | null;
  resolvedInterviewTypeLabel: string;
  resumeSuggestions: string[];
  knowledgePoints: KnowledgePointDTO[];
  onKnowledgePointSelect: (pointId: string) => void | Promise<void>;
  onKnowledgePointSkip: (pointId: string) => void | Promise<void>;
  resumePreviewError: string | null;
  resumeOpenPreviewUrl: string | null;
  numPages: number;
};

export default function InterviewResumeInsightsPanel({
  resumeScore,
  resolvedInterviewTypeLabel,
  resumeSuggestions = [],
  knowledgePoints = [],
  onKnowledgePointSelect,
  onKnowledgePointSkip,
  resumePreviewError,
  resumeOpenPreviewUrl,
  numPages,
}: InterviewResumeInsightsPanelProps) {
  return (
    <div className="h-full space-y-4 overflow-y-auto">
      <Card className="border-slate-100 p-4">
        <div className="flex items-center justify-between">
          <p className="text-sm font-medium text-slate-900">资料解析</p>
          <span className="text-2xl font-semibold text-slate-900">
            {resumeScore ?? "--"}
          </span>
        </div>
        <p className="mt-2 text-xs text-slate-500">
          {resumeScore === null
            ? "暂未返回资料解析评分"
            : "已根据资料内容完成解析"}
        </p>
      </Card>

      <Card className="space-y-3 border-slate-100 p-4">
        <p className="text-sm font-medium text-slate-900">学习主题</p>
        <Separator />
        <div className="text-sm text-slate-700">
          {resolvedInterviewTypeLabel}
        </div>
      </Card>

      <Card className="space-y-3 border-slate-100 p-4">
        <p className="text-sm font-medium text-slate-900">知识点清单</p>
        <Separator />
        <div className="space-y-3">
          {knowledgePoints.length > 0 ? (
            knowledgePoints.map((item, index) => (
              <div
                key={item.id || `${index}-${item.title}`}
                className="space-y-2 rounded-lg border border-slate-100 bg-slate-50 px-3 py-2"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 text-xs font-medium leading-5 text-slate-800">
                    {index + 1}. {item.title}
                  </div>
                  <KnowledgeStatusIcon status={item.status} />
                </div>
                {item.summary ? (
                  <div className="text-xs leading-5 text-slate-500">
                    {item.summary}
                  </div>
                ) : null}
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-7 rounded-full px-3 text-xs"
                    disabled={!item.id || item.status === "passed"}
                    onClick={() => item.id && void onKnowledgePointSelect(item.id)}
                  >
                    练习
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="h-7 rounded-full px-3 text-xs text-slate-500"
                    disabled={
                      !item.id ||
                      item.status === "passed" ||
                      item.status === "failed" ||
                      item.status === "skipped"
                    }
                    onClick={() => item.id && void onKnowledgePointSkip(item.id)}
                  >
                    跳过
                  </Button>
                  {typeof item.score === "number" ? (
                    <span className="text-[11px] text-slate-400">
                      {item.score} 分
                    </span>
                  ) : null}
                </div>
              </div>
            ))
          ) : (
            <div className="text-xs text-slate-500">暂未生成知识点清单</div>
          )}
        </div>
      </Card>

      <Card className="space-y-3 border-slate-100 p-4">
        <p className="text-sm font-medium text-slate-900">学习建议</p>
        <Separator />
        <div className="space-y-2">
          {resumeSuggestions.length > 0 ? (
            resumeSuggestions.map((item, index) => (
              <div
                key={`${index}-${item}`}
                className="text-xs leading-6 text-slate-500"
              >
                {item}
              </div>
            ))
          ) : (
            <div className="text-xs text-slate-500">暂未返回学习建议</div>
          )}
        </div>
      </Card>

      {resumePreviewError ? (
        <ErrorNotice
          title="资料预览异常"
          description={`资料预览未能正常加载：${resumePreviewError}`}
        />
      ) : null}

      {resumeOpenPreviewUrl ? (
        <Button asChild variant="outline" className="w-full">
          <a href={resumeOpenPreviewUrl} target="_blank" rel="noreferrer">
            新窗口打开资料
          </a>
        </Button>
      ) : null}

      <div className="text-right text-xs text-slate-500">
        {resumePreviewError ? "页数解析失败" : `共 ${numPages} 页`}
      </div>
    </div>
  );
}

function KnowledgeStatusIcon({ status }: { status?: string }) {
  if (status === "passed") {
    return (
      <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
        <Check className="h-3 w-3" />
      </span>
    );
  }
  if (status === "failed") {
    return (
      <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-rose-100 text-rose-700">
        <X className="h-3 w-3" />
      </span>
    );
  }
  if (status === "skipped") {
    return <span className="text-[11px] text-slate-400">已跳过</span>;
  }
  if (status === "active") {
    return <span className="text-[11px] text-sky-600">练习中</span>;
  }
  return <span className="text-[11px] text-slate-400">未练习</span>;
}
