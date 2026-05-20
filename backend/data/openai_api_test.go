package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskAiSendsDisabledThinkingForDeepSeekWhenThinkFalse(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	o := NewOpenAiFromParams(context.TODO(), srv.URL, "test-key", "deepseek-v4-pro", 0.1, 256, 10, "", false, "")
	AskAi(o, errors.New(""), []map[string]interface{}{
		{"role": "user", "content": "hello"},
	}, make(chan map[string]any, 1), "hello", false)

	thinking, ok := captured["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("expected thinking field in request body, got: %#v", captured)
	}
	if got := thinking["type"]; got != "disabled" {
		t.Fatalf("expected thinking.type=disabled, got %v", got)
	}
}

func TestPrepareMessagesForToolRequestStripsBootstrapReasoningAtDepthZero(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "assistant", "content": "bootstrap", "reasoning_content": "使用工具查询"},
		{"role": "user", "content": "继续"},
	}

	got := prepareMessagesForToolRequest(msgs, false, 0)
	if _, ok := got[0]["reasoning_content"]; ok {
		t.Fatalf("expected depth 0 tool request to strip bootstrap reasoning_content, got %#v", got[0])
	}
}

func TestPrepareMessagesForToolRequestKeepsRecursiveReasoningAtDepthOne(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "assistant", "content": "tool call", "reasoning_content": "step-1"},
		{"role": "tool", "content": "result", "tool_call_id": "call_1"},
	}

	got := prepareMessagesForToolRequest(msgs, false, 1)
	if gotReasoning, ok := got[0]["reasoning_content"]; !ok || gotReasoning != "step-1" {
		t.Fatalf("expected recursive tool request to preserve reasoning_content, got %#v", got[0])
	}
}

func TestF10GenericToMarkdownOrderedHandlesNilResult(t *testing.T) {
	got := f10GenericToMarkdownOrdered("测试标题", &F10GenericResp{}, f10LatestFinanceColOrder)
	if got == "" {
		t.Fatal("expected non-empty markdown for nil result")
	}
	if want := "暂无数据"; !strings.Contains(got, want) {
		t.Fatalf("expected markdown to contain %q, got %q", want, got)
	}
}

func TestAppendToolMessagesReusesAssistantMessageForSameTurn(t *testing.T) {
	var messages []map[string]any

	appendToolMessages(&messages, "partial answer", "reasoning", "call_1", "ToolA", `{"q":"a"}`, "result-a")
	appendToolMessages(&messages, "partial answer", "reasoning", "call_2", "ToolB", `{"q":"b"}`, "result-b")

	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (assistant + 2 tool), got %d", len(messages))
	}

	assistant := messages[0]
	toolCalls, ok := assistant["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("expected assistant tool_calls to be []map[string]any, got %#v", assistant["tool_calls"])
	}
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls on one assistant message, got %d", len(toolCalls))
	}
}

func TestNewDeepSeekOpenAiConfig(t *testing.T) {
	db.Init("../../data/stock.db")
	InitAnalyzeSentiment()

	var tools []Tool
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchStockByIndicators",
			Description: "根据自然语言筛选股票，返回自然语言选股条件要求的股票所有相关数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "选股自然语言,并且条件使用;分隔，或者条件使用,分隔。例如：创新药;PE<30;净利润增长率>50%;",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	ai := NewDeepSeekOpenAi(context.TODO(), 11)
	//res := ai.NewChatStream("长电科技", "sh600584", "长电科技分析和总结", nil)
	res := ai.NewSummaryStockNewsStreamWithTools("总结市场资讯，发掘潜力标的/行业/板块/概念，控制风险。调用工具函数验证", nil, tools, false, nil)

	for {
		select {
		case msg := <-res:
			if len(msg) > 0 {
				t.Log(msg)
				if msg["content"] == "DONE" {
					return
				}
			}
		}
	}
}

func TestGetTopNewsList(t *testing.T) {
	news := GetTopNewsList(30)
	t.Log(news)
}

func TestSearchGuShiTongStockInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	//SearchGuShiTongStockInfo("hk01810", 60)
	msgs := SearchGuShiTongStockInfo("sh600745", 60)
	for _, msg := range *msgs {
		log.SugaredLogger.Infof("%s", msg)
	}
	//SearchGuShiTongStockInfo("gb_goog", 60)

}

func TestGetZSInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	GetZSInfo("中证银行", "sz399986", 5)
	GetZSInfo("上海贝岭", "sh600171", 5)
}
