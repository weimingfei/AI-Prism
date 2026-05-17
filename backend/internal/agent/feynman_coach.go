package agent

import (
	"context"
	"strings"

	"github.com/ai-prism/backend/internal/ai"
	"github.com/cloudwego/eino/compose"
)

type FeynmanCoachWorkflow struct {
	runnable compose.Runnable[FeynmanCoachInput, FeynmanCoachOutput]
	client   ai.Client
	model    string
}

type FeynmanCoachInput struct {
	SessionID    string
	Explanation  string
	FollowUpMode string
}

type FeynmanCoachOutput struct {
	Diagnosis    Diagnosis
	FollowUp     FollowUp
	Correction   string
	MasteryScore int
	Intent       string
}

type Diagnosis struct {
	CorrectParts   []string
	MissingParts   []string
	Misconceptions []string
	Clarity        string
}

type FollowUp struct {
	Question string
	Reason   string
	Targets  []string
}

func NewFeynmanCoachWorkflow(client ai.Client, model string) *FeynmanCoachWorkflow {
	workflow := &FeynmanCoachWorkflow{
		client: client,
		model:  strings.TrimSpace(model),
	}
	chain := compose.NewChain[FeynmanCoachInput, FeynmanCoachOutput]().
		AppendLambda(compose.InvokableLambda(func(ctx context.Context, input FeynmanCoachInput) (FeynmanCoachOutput, error) {
			return workflow.diagnose(ctx, input)
		}))

	runnable, err := chain.Compile(context.Background(), compose.WithGraphName("feynman_coach_mvp"))
	if err != nil {
		return workflow
	}

	workflow.runnable = runnable
	return workflow
}

func (w *FeynmanCoachWorkflow) Diagnose(ctx context.Context, input FeynmanCoachInput) FeynmanCoachOutput {
	if w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil {
			return output
		}
	}
	return diagnoseWithHeuristics(input)
}

func (w *FeynmanCoachWorkflow) diagnose(ctx context.Context, input FeynmanCoachInput) (FeynmanCoachOutput, error) {
	if w.client == nil || w.model == "" {
		return diagnoseWithHeuristics(input), nil
	}

	response, err := w.client.Chat(ai.WithSingleFlightStage(ctx, "interview-evaluation"), ai.ChatRequest{
		Model: w.model,
		Messages: []ai.Message{
			{
				Role:    "system",
				Content: feynmanCoachSystemPrompt(),
			},
			{
				Role:    "user",
				Content: strings.TrimSpace(input.Explanation) + "\n\n追问模式：" + followUpModeInstruction(input.FollowUpMode),
			},
		},
	})
	if err != nil {
		return diagnoseWithHeuristics(input), nil
	}

	output, err := parseCoachOutput(response.Content)
	if err != nil {
		return diagnoseWithHeuristics(input), nil
	}
	return output, nil
}

func feynmanCoachSystemPrompt() string {
	return `你是 AI 棱镜的知识讲解练习助手。请根据用户对知识点的讲解，输出严格 JSON，不要输出 Markdown 或额外解释。
JSON 结构：
{
  "intent": "answer|unknown|show_reference",
  "diagnosis": {
    "correctParts": ["已经讲清楚的点"],
    "missingParts": ["缺失或需要补充的点"],
    "misconceptions": ["可能误区"],
    "clarity": "clear|partially_clear|unclear"
  },
  "followUp": {
    "question": "下一句追问",
    "reason": "为什么追问",
    "targets": ["追问目标"]
  },
  "correction": "修正版解释建议",
  "masteryScore": 0
}
intent 说明：
- answer：用户正在尝试回答或讲解。
- unknown：用户表达不知道、不会、忘记、没思路、无法继续等。
- show_reference：用户明确想直接看完整答案或参考答案。
masteryScore 必须是 0 到 100 的整数。`
}

func followUpModeInstruction(mode string) string {
	switch strings.TrimSpace(mode) {
	case "divergent":
		return "发散模式。追问可以围绕知识点拓展到相关领域、典型应用、对比概念或模型自带背景知识，但必须服务于当前知识点理解。"
	default:
		return "边界模式。追问只能基于用户上传资料或当前题目范围，不要引入资料外的新知识。"
	}
}

func parseCoachOutput(content string) (FeynmanCoachOutput, error) {
	var output FeynmanCoachOutput
	if err := decodeAgentJSON(content, &output, "diagnosis", "masteryScore", "followUp"); err != nil {
		return FeynmanCoachOutput{}, err
	}
	if output.MasteryScore < 0 {
		output.MasteryScore = 0
	}
	if output.MasteryScore > 100 {
		output.MasteryScore = 100
	}
	if strings.TrimSpace(output.Diagnosis.Clarity) == "" {
		output.Diagnosis.Clarity = clarity(output.MasteryScore)
	}
	if strings.TrimSpace(output.Correction) == "" {
		output.Correction = "建议按“定义 -> 例子 -> 反例 -> 边界”的结构补充讲解。"
	}
	if strings.TrimSpace(output.FollowUp.Question) == "" {
		output.FollowUp.Question = "请补充一个反例，并说明它为什么不适用当前概念。"
	}
	output.Intent = normalizeCoachIntent(output.Intent)
	return output, nil
}

func diagnoseWithHeuristics(input FeynmanCoachInput) FeynmanCoachOutput {
	explanation := strings.TrimSpace(input.Explanation)
	score := heuristicScore(explanation)

	missing := []string{"请补充概念的边界条件", "请给出一个反例或不适用场景"}
	if len([]rune(explanation)) > 120 {
		missing = []string{"请进一步压缩成初学者也能听懂的版本"}
	}

	return FeynmanCoachOutput{
		Intent: detectCoachIntent(explanation),
		Diagnosis: Diagnosis{
			CorrectParts:   []string{"已经尝试用自己的语言组织解释"},
			MissingParts:   missing,
			Misconceptions: []string{"当前骨架尚未接入真实模型，误区识别为占位结果"},
			Clarity:        clarity(score),
		},
		FollowUp: FollowUp{
			Question: "如果把这个知识点教给完全没有背景的人，你会用什么生活例子解释？这个例子和原概念有哪些不完全一致的地方？",
			Reason:   "讲解练习需要用例子暴露概念边界。",
			Targets:  []string{"生活例子", "概念边界", "反例"},
		},
		Correction:   "建议按“定义 -> 为什么重要 -> 一个例子 -> 一个反例 -> 使用边界”的顺序重新讲解。",
		MasteryScore: score,
	}
}

func normalizeCoachIntent(intent string) string {
	switch strings.TrimSpace(strings.ToLower(intent)) {
	case "unknown", "show_reference":
		return strings.TrimSpace(strings.ToLower(intent))
	default:
		return "answer"
	}
}

func detectCoachIntent(explanation string) string {
	normalized := strings.TrimSpace(strings.ToLower(explanation))
	if normalized == "" {
		return "unknown"
	}
	referencePhrases := []string{"完整答案", "参考答案", "直接给答案", "给我答案", "show answer", "reference answer"}
	for _, phrase := range referencePhrases {
		if strings.Contains(normalized, phrase) {
			return "show_reference"
		}
	}
	unknownPhrases := []string{
		"不会", "不知道", "不清楚", "忘了", "我忘了", "没思路", "不会答", "不知道答案",
		"i don't know", "dont know", "do not know", "no idea",
	}
	for _, phrase := range unknownPhrases {
		if strings.Contains(normalized, phrase) {
			return "unknown"
		}
	}
	return "answer"
}

func heuristicScore(explanation string) int {
	length := len([]rune(explanation))
	switch {
	case length >= 240:
		return 78
	case length >= 120:
		return 68
	case length >= 40:
		return 55
	default:
		return 35
	}
}

func clarity(score int) string {
	switch {
	case score >= 80:
		return "clear"
	case score >= 60:
		return "partially_clear"
	default:
		return "unclear"
	}
}
