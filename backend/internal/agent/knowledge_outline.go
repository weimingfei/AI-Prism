package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ai-prism/backend/internal/ai"
	"github.com/cloudwego/eino/compose"
)

type KnowledgeOutlineWorkflow struct {
	runnable compose.Runnable[KnowledgeOutlineInput, KnowledgeOutlineOutput]
	client   ai.Client
	model    string
	parser   DocumentParserConfig
}

type KnowledgeOutlineInput struct {
	FileName string
	Content  []byte
}

type KnowledgeOutlineOutput struct {
	Title       string           `json:"title"`
	Summary     string           `json:"summary"`
	Points      []KnowledgePoint `json:"points"`
	Suggestions []string         `json:"suggestions"`
	Score       int              `json:"score"`
}

type KnowledgePoint struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Keywords    []string `json:"keywords"`
	Difficulty  string   `json:"difficulty"`
	CheckPrompt string   `json:"checkPrompt"`
}

type DocumentParserConfig struct {
	MinerU MinerUConfig
}

type MinerUConfig struct {
	Enabled        bool
	BaseURL        string
	APIKey         string
	ParseEndpoint  string
	TimeoutSeconds int
}

type ParsedDocument struct {
	Text    string
	Format  string
	Quality DocumentQuality
}

type DocumentQuality struct {
	OK             bool
	Empty          bool
	Garbled        bool
	TableRisk      bool
	FormulaRisk    bool
	ReadableRunes  int
	Reason         string
	NeedEnhanced   bool
	EnhancedEngine string
}

func NewKnowledgeOutlineWorkflow(client ai.Client, model string, parser ...DocumentParserConfig) *KnowledgeOutlineWorkflow {
	workflow := &KnowledgeOutlineWorkflow{
		client: client,
		model:  strings.TrimSpace(model),
	}
	if len(parser) > 0 {
		workflow.parser = parser[0]
	}
	chain := compose.NewChain[KnowledgeOutlineInput, KnowledgeOutlineOutput]().
		AppendLambda(compose.InvokableLambda(func(ctx context.Context, input KnowledgeOutlineInput) (KnowledgeOutlineOutput, error) {
			return workflow.generate(ctx, input)
		}))

	runnable, err := chain.Compile(context.Background(), compose.WithGraphName("knowledge_outline"))
	if err != nil {
		return workflow
	}
	workflow.runnable = runnable
	return workflow
}

func (w *KnowledgeOutlineWorkflow) Generate(ctx context.Context, input KnowledgeOutlineInput) KnowledgeOutlineOutput {
	if w.runnable != nil {
		output, err := w.runnable.Invoke(ctx, input)
		if err == nil && len(output.Points) > 0 {
			return output
		}
	}
	return fallbackKnowledgeOutline(input)
}

func (w *KnowledgeOutlineWorkflow) generate(ctx context.Context, input KnowledgeOutlineInput) (KnowledgeOutlineOutput, error) {
	parsed := w.parseDocument(ctx, input)
	documentText := parsed.Text
	if w.client == nil || w.model == "" {
		return fallbackKnowledgeOutlineWithText(input.FileName, documentText), nil
	}

	response, err := w.client.Chat(ai.WithSingleFlightStage(ctx, "interview-extraction"), ai.ChatRequest{
		Model: w.model,
		Messages: []ai.Message{
			{
				Role:    "system",
				Content: knowledgeOutlineSystemPrompt(),
			},
			{
				Role: "user",
				Content: fmt.Sprintf(`文件名：%s

文档可读文本：
%s`, input.FileName, truncateRunes(documentText, 12000)),
			},
		},
	})
	if err != nil {
		return fallbackKnowledgeOutlineWithText(input.FileName, documentText), nil
	}

	output, err := parseKnowledgeOutline(response.Content)
	if err != nil || len(output.Points) == 0 {
		return fallbackKnowledgeOutlineWithText(input.FileName, documentText), nil
	}
	output.Score = clampScore(output.Score)
	return output, nil
}

func knowledgeOutlineSystemPrompt() string {
	return `你是 AI 棱镜系统的“资料解析与知识点大纲 Agent”。
任务：根据用户上传资料的可读文本，提炼适合讲解练习的知识点清单。

要求：
1. 输出严格 JSON，不要输出 Markdown，不要解释。
2. 知识点应可用于后续“用户讲解 -> AI 追问 -> 纠错评分”。
3. 每个知识点必须包含标题、摘要、关键词、难度、自测讲解提示。
4. 如果文本不足，也要根据文件名和可读片段生成保守的大纲。

JSON 结构：
{
  "title": "资料主题",
  "summary": "资料整体摘要",
  "points": [
    {
      "id": "kp-1",
      "title": "知识点标题",
      "summary": "知识点摘要",
      "keywords": ["关键词"],
      "difficulty": "easy|medium|hard",
      "checkPrompt": "请用户用自己的话讲解的问题"
    }
  ],
  "suggestions": ["学习建议"],
  "score": 0
}
score 表示资料可解析质量，0 到 100。points 生成 3 到 8 个。`
}

func parseKnowledgeOutline(content string) (KnowledgeOutlineOutput, error) {
	var output KnowledgeOutlineOutput
	if err := decodeAgentJSON(content, &output, "points", "title"); err != nil {
		return KnowledgeOutlineOutput{}, err
	}
	for index := range output.Points {
		if strings.TrimSpace(output.Points[index].ID) == "" {
			output.Points[index].ID = fmt.Sprintf("kp-%d", index+1)
		}
		if strings.TrimSpace(output.Points[index].Difficulty) == "" {
			output.Points[index].Difficulty = "medium"
		}
		if strings.TrimSpace(output.Points[index].CheckPrompt) == "" {
			output.Points[index].CheckPrompt = fmt.Sprintf("请用自己的话讲清楚“%s”，并举一个例子和一个反例。", output.Points[index].Title)
		}
	}
	output.Score = clampScore(output.Score)
	return output, nil
}

func extractReadableDocumentText(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	if utf8.Valid(content) {
		return normalizeWhitespace(string(content))
	}

	raw := string(content)
	matches := regexp.MustCompile(`\(([^()]|\\.){6,}\)`).FindAllString(raw, 300)
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		value := strings.TrimPrefix(strings.TrimSuffix(match, ")"), "(")
		value = strings.ReplaceAll(value, `\(`, "(")
		value = strings.ReplaceAll(value, `\)`, ")")
		value = strings.ReplaceAll(value, `\\`, `\`)
		value = normalizeWhitespace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n")
}

func (w *KnowledgeOutlineWorkflow) parseDocument(ctx context.Context, input KnowledgeOutlineInput) ParsedDocument {
	text := extractReadableDocumentText(input.Content)
	quality := detectDocumentQuality(text, input.Content)
	if quality.NeedEnhanced {
		enhanced := w.parseDocumentWithMinerU(ctx, input)
		if strings.TrimSpace(enhanced.Text) != "" {
			return enhanced
		}
	}
	return ParsedDocument{
		Text:    text,
		Format:  "text",
		Quality: quality,
	}
}

func detectDocumentQuality(text string, raw []byte) DocumentQuality {
	normalized := strings.TrimSpace(text)
	readableRunes := len([]rune(normalized))
	quality := DocumentQuality{
		OK:            true,
		ReadableRunes: readableRunes,
	}
	if readableRunes < 80 {
		quality.OK = false
		quality.Empty = readableRunes == 0
		quality.NeedEnhanced = true
		quality.Reason = "文本层为空或内容过少"
	}
	replacementCount := strings.Count(normalized, "�")
	if replacementCount > 0 && replacementCount*10 > readableRunes {
		quality.OK = false
		quality.Garbled = true
		quality.NeedEnhanced = true
		quality.Reason = "文本存在明显乱码"
	}
	rawString := string(raw)
	if strings.Contains(rawString, "/Table") || strings.Contains(rawString, "/StructTreeRoot") {
		quality.TableRisk = true
	}
	if strings.Contains(rawString, "/Formula") || strings.Contains(rawString, "Math") {
		quality.FormulaRisk = true
	}
	return quality
}

func (w *KnowledgeOutlineWorkflow) parseDocumentWithMinerU(ctx context.Context, input KnowledgeOutlineInput) ParsedDocument {
	cfg := w.parser.MinerU
	if !cfg.Enabled || strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return minerUFailure("MinerU API 尚未配置")
	}

	endpoint := strings.TrimSpace(cfg.ParseEndpoint)
	if endpoint == "" {
		endpoint = "/api/v1/pdf/parse"
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", input.FileName)
	if err != nil {
		return minerUFailure("创建 MinerU 上传请求失败")
	}
	if _, err := part.Write(input.Content); err != nil {
		return minerUFailure("写入 MinerU 上传文件失败")
	}
	_ = writer.WriteField("return_format", "markdown")
	if err := writer.Close(); err != nil {
		return minerUFailure("关闭 MinerU 上传请求失败")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+endpoint, &body)
	if err != nil {
		return minerUFailure("创建 MinerU HTTP 请求失败")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return minerUFailure("调用 MinerU 失败")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return minerUFailure("读取 MinerU 响应失败")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return minerUFailure(fmt.Sprintf("MinerU 返回状态码 %d", resp.StatusCode))
	}

	text := extractMinerUText(respBody)
	if strings.TrimSpace(text) == "" {
		return minerUFailure("MinerU 响应未包含可读 Markdown 或文本")
	}

	return ParsedDocument{
		Text:   normalizeWhitespace(text),
		Format: "mineru",
		Quality: DocumentQuality{
			OK:             true,
			ReadableRunes:  len([]rune(text)),
			EnhancedEngine: "mineru",
		},
	}
}

func fallbackKnowledgeOutline(input KnowledgeOutlineInput) KnowledgeOutlineOutput {
	return fallbackKnowledgeOutlineWithText(input.FileName, extractReadableDocumentText(input.Content))
}

func minerUFailure(reason string) ParsedDocument {
	return ParsedDocument{
		Format: "mineru",
		Quality: DocumentQuality{
			OK:             false,
			NeedEnhanced:   true,
			EnhancedEngine: "mineru",
			Reason:         reason,
		},
	}
}

func extractMinerUText(content []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return string(content)
	}
	for _, key := range []string{"markdown", "md", "text", "content"} {
		if value, ok := payload[key].(string); ok {
			return value
		}
	}
	for _, key := range []string{"data", "result"} {
		nested, ok := payload[key].(map[string]any)
		if !ok {
			continue
		}
		for _, nestedKey := range []string{"markdown", "md", "text", "content"} {
			if value, ok := nested[nestedKey].(string); ok {
				return value
			}
		}
	}
	return ""
}

func fallbackKnowledgeOutlineWithText(fileName string, text string) KnowledgeOutlineOutput {
	title := strings.TrimSuffix(strings.TrimSpace(fileName), ".pdf")
	if title == "" {
		title = "上传资料"
	}
	keywords := []string{"定义", "例子", "边界"}
	if text != "" {
		keywords = topKeywords(text)
	}
	return KnowledgeOutlineOutput{
		Title:   title,
		Summary: "已根据上传资料生成初始学习大纲，请从第一个知识点开始用自己的话讲解。",
		Points: []KnowledgePoint{
			{
				ID:          "kp-1",
				Title:       title + "的核心概念",
				Summary:     "解释该主题的基本定义、为什么重要以及主要使用场景。",
				Keywords:    keywords,
				Difficulty:  "medium",
				CheckPrompt: "请用自己的话讲清楚这个知识点，并举一个初学者能理解的例子。",
			},
			{
				ID:          "kp-2",
				Title:       title + "的关键边界",
				Summary:     "说明该知识点适用与不适用的条件，找出容易误解的地方。",
				Keywords:    []string{"边界", "反例", "误区"},
				Difficulty:  "medium",
				CheckPrompt: "请给出一个反例，说明这个知识点在哪些场景下不适用。",
			},
			{
				ID:          "kp-3",
				Title:       title + "的迁移应用",
				Summary:     "把知识点迁移到新的例子或问题中，检验是否真正理解。",
				Keywords:    []string{"应用", "迁移", "复述"},
				Difficulty:  "hard",
				CheckPrompt: "请换一个新场景重新解释这个知识点，并说明推理过程。",
			},
		},
		Suggestions: []string{
			"先选择一个知识点，用 3 到 5 句话完成第一版讲解。",
			"讲解时必须包含定义、例子、反例和边界。",
			"讲完后让 AI 追问你最不确定的一点。",
		},
		Score: 60,
	}
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func topKeywords(text string) []string {
	words := strings.Fields(text)
	result := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, word := range words {
		word = strings.Trim(word, "，。,.：:；;（）()[]【】")
		if len([]rune(word)) < 2 || len([]rune(word)) > 12 {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
		if len(result) == 3 {
			return result
		}
	}
	return []string{"定义", "例子", "边界"}
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
