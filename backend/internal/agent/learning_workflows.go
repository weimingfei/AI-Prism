package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ai-prism/backend/internal/ai"
	"github.com/cloudwego/eino/compose"
)

type DocumentQualityWorkflow struct {
	runnable compose.Runnable[DocumentQualityInput, DocumentQualityOutput]
}

type DocumentQualityInput struct {
	FileName string `json:"fileName"`
	Text     string `json:"text"`
}

type DocumentQualityOutput struct {
	OK           bool     `json:"ok"`
	NeedMinerU   bool     `json:"needMinerU"`
	Issues       []string `json:"issues"`
	ReadableSize int      `json:"readableSize"`
}

type QuestionGeneratorWorkflow struct {
	runnable compose.Runnable[QuestionGeneratorInput, QuestionGeneratorOutput]
	client   ai.Client
	model    string
}

type QuestionGeneratorInput struct {
	Title   string           `json:"title"`
	Summary string           `json:"summary"`
	Points  []KnowledgePoint `json:"points"`
}

type QuestionGeneratorOutput struct {
	Questions   map[string]string `json:"questions"`
	Suggestions []string          `json:"suggestions"`
}

type AnswerScoringWorkflow struct {
	runnable compose.Runnable[AnswerScoringInput, AnswerScoringOutput]
	client   ai.Client
	model    string
}

type AnswerScoringInput struct {
	SessionID        string `json:"sessionId"`
	QuestionNumber   string `json:"questionNumber"`
	Question         string `json:"question"`
	Answer           string `json:"answer"`
	KnowledgeContext string `json:"knowledgeContext"`
}

type AnswerScoringOutput struct {
	Score          int      `json:"score"`
	LogicOK        bool     `json:"logicOk"`
	MissingPoints  []string `json:"missingPoints"`
	Feedback       string   `json:"feedback"`
	FollowUpNeeded bool     `json:"followUpNeeded"`
}

type FollowUpQuestionWorkflow struct {
	runnable compose.Runnable[FollowUpQuestionInput, FollowUpQuestionOutput]
	client   ai.Client
	model    string
}

type FollowUpQuestionInput struct {
	Question      string   `json:"question"`
	Answer        string   `json:"answer"`
	MissingPoints []string `json:"missingPoints"`
	CurrentRound  int      `json:"currentRound"`
	MaxRound      int      `json:"maxRound"`
	Mode          string   `json:"mode"`
	MaterialScope string   `json:"materialScope"`
}

type FollowUpQuestionOutput struct {
	AskToUser   string   `json:"askToUser"`
	EndPractice bool     `json:"endPractice"`
	Targets     []string `json:"targets"`
	Reason      string   `json:"reason"`
}

type LearningCardReviewWorkflow struct {
	runnable compose.Runnable[LearningCardReviewInput, LearningCardReviewOutput]
	client   ai.Client
	model    string
}

type LearningCardReviewInput struct {
	KnowledgeTitle string               `json:"knowledgeTitle"`
	Question       string               `json:"question"`
	Answer         string               `json:"answer"`
	Feedback       string               `json:"feedback"`
	Score          int                  `json:"score"`
	Turns          []LearningReviewTurn `json:"turns"`
}

type LearningReviewTurn struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

type LearningCardReviewOutput struct {
	MarkdownCard   string       `json:"markdownCard"`
	ReviewPlan     []ReviewTask `json:"reviewPlan"`
	OverallComment string       `json:"overallComment"`
	Highlights     []string     `json:"highlights"`
	Improvements   []string     `json:"improvements"`
}

type ReviewTask struct {
	When   string `json:"when"`
	Focus  string `json:"focus"`
	Task   string `json:"task"`
	Target string `json:"target"`
}

func NewDocumentQualityWorkflow() *DocumentQualityWorkflow {
	workflow := &DocumentQualityWorkflow{}
	runnable, err := compileSingleLambdaGraph("document_quality_gate", "inspect_quality", func(ctx context.Context, input DocumentQualityInput) (DocumentQualityOutput, error) {
		return inspectDocumentQuality(input), nil
	})
	if err == nil {
		workflow.runnable = runnable
	}
	return workflow
}

func (w *DocumentQualityWorkflow) Inspect(ctx context.Context, input DocumentQualityInput) DocumentQualityOutput {
	if w != nil && w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil {
			return output
		}
	}
	return inspectDocumentQuality(input)
}

func NewQuestionGeneratorWorkflow(client ai.Client, model string) *QuestionGeneratorWorkflow {
	workflow := &QuestionGeneratorWorkflow{client: client, model: strings.TrimSpace(model)}
	runnable, err := compileSingleLambdaGraph("knowledge_question_generator", "generate_questions", func(ctx context.Context, input QuestionGeneratorInput) (QuestionGeneratorOutput, error) {
		return workflow.generate(ctx, input)
	})
	if err == nil {
		workflow.runnable = runnable
	}
	return workflow
}

func (w *QuestionGeneratorWorkflow) Generate(ctx context.Context, input QuestionGeneratorInput) QuestionGeneratorOutput {
	if w != nil && w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil && len(output.Questions) > 0 {
			return output
		}
	}
	return fallbackQuestionGenerator(input)
}

func NewAnswerScoringWorkflow(client ai.Client, model string) *AnswerScoringWorkflow {
	workflow := &AnswerScoringWorkflow{client: client, model: strings.TrimSpace(model)}
	runnable, err := compileSingleLambdaGraph("answer_scoring_agent", "score_answer", func(ctx context.Context, input AnswerScoringInput) (AnswerScoringOutput, error) {
		return workflow.score(ctx, input)
	})
	if err == nil {
		workflow.runnable = runnable
	}
	return workflow
}

func (w *AnswerScoringWorkflow) Score(ctx context.Context, input AnswerScoringInput) AnswerScoringOutput {
	if w != nil && w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil {
			output.Score = clampScore(output.Score)
			return output
		}
	}
	return fallbackAnswerScore(input)
}

func NewFollowUpQuestionWorkflow(client ai.Client, model string) *FollowUpQuestionWorkflow {
	workflow := &FollowUpQuestionWorkflow{client: client, model: strings.TrimSpace(model)}
	runnable, err := compileSingleLambdaGraph("followup_question_agent", "generate_followup", func(ctx context.Context, input FollowUpQuestionInput) (FollowUpQuestionOutput, error) {
		return workflow.generate(ctx, input)
	})
	if err == nil {
		workflow.runnable = runnable
	}
	return workflow
}

func (w *FollowUpQuestionWorkflow) Generate(ctx context.Context, input FollowUpQuestionInput) FollowUpQuestionOutput {
	if w != nil && w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil && (output.EndPractice || strings.TrimSpace(output.AskToUser) != "") {
			return output
		}
	}
	return fallbackFollowUpQuestion(input)
}

func NewLearningCardReviewWorkflow(client ai.Client, model string) *LearningCardReviewWorkflow {
	workflow := &LearningCardReviewWorkflow{client: client, model: strings.TrimSpace(model)}
	runnable, err := compileSingleLambdaGraph("learning_card_review_agent", "generate_card_review", func(ctx context.Context, input LearningCardReviewInput) (LearningCardReviewOutput, error) {
		return workflow.generate(ctx, input)
	})
	if err == nil {
		workflow.runnable = runnable
	}
	return workflow
}

func (w *LearningCardReviewWorkflow) Generate(ctx context.Context, input LearningCardReviewInput) LearningCardReviewOutput {
	if w != nil && w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil && strings.TrimSpace(output.MarkdownCard) != "" {
			return output
		}
	}
	return fallbackLearningCardReview(input)
}

func (w *QuestionGeneratorWorkflow) generate(ctx context.Context, input QuestionGeneratorInput) (QuestionGeneratorOutput, error) {
	if w.client == nil || w.model == "" {
		return fallbackQuestionGenerator(input), nil
	}
	content, err := callJSONAgent(ctx, w.client, w.model, questionGeneratorPrompt(), input)
	if err != nil {
		return fallbackQuestionGenerator(input), nil
	}
	var output QuestionGeneratorOutput
	if err := decodeJSON(content, &output); err != nil || len(output.Questions) == 0 {
		return fallbackQuestionGenerator(input), nil
	}
	return output, nil
}

func (w *AnswerScoringWorkflow) score(ctx context.Context, input AnswerScoringInput) (AnswerScoringOutput, error) {
	if w.client == nil || w.model == "" {
		return fallbackAnswerScore(input), nil
	}
	content, err := callJSONAgent(ctx, w.client, w.model, answerScoringPrompt(), input)
	if err != nil {
		return fallbackAnswerScore(input), nil
	}
	var output AnswerScoringOutput
	if err := decodeJSON(content, &output); err != nil {
		return fallbackAnswerScore(input), nil
	}
	output.Score = clampScore(output.Score)
	return output, nil
}

func (w *FollowUpQuestionWorkflow) generate(ctx context.Context, input FollowUpQuestionInput) (FollowUpQuestionOutput, error) {
	if input.CurrentRound >= input.MaxRound {
		return FollowUpQuestionOutput{EndPractice: true, Reason: "已达到追问次数上限"}, nil
	}
	if w.client == nil || w.model == "" {
		return fallbackFollowUpQuestion(input), nil
	}
	content, err := callJSONAgent(ctx, w.client, w.model, followUpQuestionPrompt(), input)
	if err != nil {
		return fallbackFollowUpQuestion(input), nil
	}
	var output FollowUpQuestionOutput
	if err := decodeJSON(content, &output); err != nil {
		return fallbackFollowUpQuestion(input), nil
	}
	return output, nil
}

func (w *LearningCardReviewWorkflow) generate(ctx context.Context, input LearningCardReviewInput) (LearningCardReviewOutput, error) {
	if w.client == nil || w.model == "" {
		return fallbackLearningCardReview(input), nil
	}
	content, err := callJSONAgent(ctx, w.client, w.model, learningCardReviewPrompt(), input)
	if err != nil {
		return fallbackLearningCardReview(input), nil
	}
	var output LearningCardReviewOutput
	if err := decodeJSON(content, &output); err != nil || strings.TrimSpace(output.MarkdownCard) == "" {
		return fallbackLearningCardReview(input), nil
	}
	return output, nil
}

func RegisterDevWorkflows(client ai.Client, model string) {
	_ = NewDocumentQualityWorkflow()
	_ = NewQuestionGeneratorWorkflow(client, model)
	_ = NewAnswerScoringWorkflow(client, model)
	_ = NewFollowUpQuestionWorkflow(client, model)
	_ = NewLearningCardReviewWorkflow(client, model)
}

func callJSONAgent(ctx context.Context, client ai.Client, model string, systemPrompt string, input any) (string, error) {
	data, _ := json.Marshal(input)
	ctx = ai.WithSingleFlightStage(ctx, singleFlightStageFromPrompt(systemPrompt))
	response, err := client.Chat(ctx, ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(data)},
		},
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

func singleFlightStageFromPrompt(prompt string) string {
	switch {
	case strings.Contains(prompt, "追问"):
		return "interview-followup"
	case strings.Contains(prompt, "评分"), strings.Contains(prompt, "评价"):
		return "interview-evaluation"
	case strings.Contains(prompt, "知识卡片"), strings.Contains(prompt, "复习计划"):
		return "learning-card-review"
	case strings.Contains(prompt, "问题"):
		return "knowledge-question-generator"
	default:
		return "ai-agent-json"
	}
}

func compileSingleLambdaGraph[I, O any](graphName string, nodeName string, fn func(context.Context, I) (O, error)) (compose.Runnable[I, O], error) {
	graph := compose.NewGraph[I, O]()
	if err := graph.AddLambdaNode(nodeName, compose.InvokableLambda(fn)); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(compose.START, nodeName); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(nodeName, compose.END); err != nil {
		return nil, err
	}
	return graph.Compile(context.Background(), compose.WithGraphName(graphName))
}

func decodeJSON(content string, output any) error {
	return decodeAgentJSON(content, output)
}

func inspectDocumentQuality(input DocumentQualityInput) DocumentQualityOutput {
	text := strings.TrimSpace(input.Text)
	issues := []string{}
	if len([]rune(text)) < 80 {
		issues = append(issues, "文本层为空或过短")
	}
	if strings.Count(text, "�") > 3 {
		issues = append(issues, "疑似乱码")
	}
	if strings.Count(text, "|") > 10 {
		issues = append(issues, "表格结构可能丢失")
	}
	needMinerU := len(issues) > 0
	return DocumentQualityOutput{
		OK:           !needMinerU,
		NeedMinerU:   needMinerU,
		Issues:       issues,
		ReadableSize: len([]rune(text)),
	}
}

func questionGeneratorPrompt() string {
	return `你是“知识点题目生成 Agent”。根据知识点大纲生成每个知识点的讲解练习题。
输出严格 JSON：{"questions":{"1":"问题"},"suggestions":["建议"]}。问题要能引导用户讲定义、例子、边界和反例。`
}

func answerScoringPrompt() string {
	return `你是“讲解评分 Agent”。根据问题、资料上下文和用户回答评分。
输出严格 JSON：{"score":0,"logicOk":true,"missingPoints":["缺失点"],"feedback":"反馈","followUpNeeded":true}。
评分标准：概念准确、结构清晰、例子有效、边界/反例充分。`
}

func followUpQuestionPrompt() string {
	return `你是“追问生成 Agent”。根据用户回答和缺失点生成下一次追问。
输出严格 JSON：{"askToUser":"追问","endPractice":false,"targets":["目标"],"reason":"原因"}。
边界模式只问资料范围内问题；发散模式可拓展相关知识但必须服务当前知识点。若已达到上限或无需追问，endPractice=true。`
}

func learningCardReviewPrompt() string {
	return `你是“知识卡片与复习计划 Agent”。根据本轮问答生成 Markdown 知识卡片和复习计划。
输出严格 JSON：{"markdownCard":"# 标题...","reviewPlan":[{"when":"today","focus":"...","task":"...","target":"..."}],"overallComment":"...","highlights":["..."],"improvements":["..."]}。`
}

func fallbackQuestionGenerator(input QuestionGeneratorInput) QuestionGeneratorOutput {
	questions := map[string]string{}
	for index, point := range input.Points {
		prompt := strings.TrimSpace(point.CheckPrompt)
		if prompt == "" {
			prompt = fmt.Sprintf("请用自己的话讲清楚“%s”，并举一个例子和一个反例。", point.Title)
		}
		questions[fmt.Sprintf("%d", index+1)] = prompt
	}
	if len(questions) == 0 {
		questions["1"] = "请用自己的话讲清楚该资料的核心概念，并举一个初学者能理解的例子。"
	}
	return QuestionGeneratorOutput{
		Questions: questions,
		Suggestions: []string{
			"先讲定义，再讲例子，最后讲边界。",
			"每个知识点至少准备一个反例。",
		},
	}
}

func fallbackAnswerScore(input AnswerScoringInput) AnswerScoringOutput {
	score := heuristicScore(input.Answer)
	missing := []string{"概念边界", "反例或不适用场景"}
	if len([]rune(input.Answer)) > 140 {
		missing = []string{"压缩表达", "突出最核心因果关系"}
	}
	return AnswerScoringOutput{
		Score:          score,
		LogicOK:        score >= 70,
		MissingPoints:  missing,
		Feedback:       "建议按“定义 -> 例子 -> 反例 -> 边界”的结构补充讲解。",
		FollowUpNeeded: score < 85,
	}
}

func fallbackFollowUpQuestion(input FollowUpQuestionInput) FollowUpQuestionOutput {
	if input.CurrentRound >= input.MaxRound {
		return FollowUpQuestionOutput{EndPractice: true, Reason: "已达到追问次数上限"}
	}
	target := "概念边界"
	if len(input.MissingPoints) > 0 {
		target = input.MissingPoints[0]
	}
	return FollowUpQuestionOutput{
		AskToUser:   fmt.Sprintf("请围绕“%s”再补充一个具体例子，并说明它和原概念的边界。", target),
		EndPractice: false,
		Targets:     []string{target},
		Reason:      "当前讲解还需要通过例子暴露理解边界。",
	}
}

func fallbackLearningCardReview(input LearningCardReviewInput) LearningCardReviewOutput {
	title := strings.TrimSpace(input.KnowledgeTitle)
	if title == "" {
		title = "AI 棱镜知识卡片"
	}
	card := fmt.Sprintf(`# %s

## 我的解释

%s

## AI 反馈

%s

## 当前掌握度

%d/100
`, title, strings.TrimSpace(input.Answer), strings.TrimSpace(input.Feedback), clampScore(input.Score))
	return LearningCardReviewOutput{
		MarkdownCard: card,
		ReviewPlan: []ReviewTask{
			{When: "today", Focus: "补齐定义和边界", Task: "用 3 句话重新讲解并补一个反例。", Target: "能讲清适用和不适用场景。"},
			{When: "tomorrow", Focus: "迁移应用", Task: "换一个新例子解释同一知识点。", Target: "能脱离原资料复述。"},
			{When: "day_3", Focus: "主动回忆", Task: "不看笔记完成一次口头讲解。", Target: "追问回答完整率达到 80%。"},
		},
		OverallComment: "本轮学习记录已生成，建议继续围绕概念边界、反例和迁移应用复习。",
		Highlights:     []string{"已经开始用自己的话讲解"},
		Improvements:   []string{"补充反例", "压缩成初学者能听懂的版本"},
	}
}
