package agent

import "testing"

func TestDecodeAgentJSONExtractsFirstBalancedObject(t *testing.T) {
	var out AnswerScoringOutput
	input := "前置解释 {\"score\":\"82\",\"logic_ok\":\"true\",\"missing_points\":[\"边界\"],\"feedback\":\"可以\",\"follow_up_needed\":\"false\"} 后置解释 {bad}"
	if err := decodeAgentJSON(input, &out, "score"); err != nil {
		t.Fatalf("decodeAgentJSON error: %v", err)
	}
	if out.Score != 82 || !out.LogicOK || out.FollowUpNeeded {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestDecodeAgentJSONUnwrapsJSONStringField(t *testing.T) {
	var out FeynmanCoachOutput
	input := "{\"json\":\"{\\\"mastery_score\\\":77,\\\"diagnosis\\\":{\\\"correct_parts\\\":[\\\"A\\\"],\\\"missing_parts\\\":[\\\"B\\\"],\\\"misconceptions\\\":[],\\\"clarity\\\":\\\"clear\\\"},\\\"follow_up\\\":{\\\"question\\\":\\\"Q\\\",\\\"reason\\\":\\\"R\\\",\\\"targets\\\":[\\\"T\\\"]},\\\"correction\\\":\\\"C\\\"}\"}"
	if err := decodeAgentJSON(input, &out, "masteryScore", "diagnosis"); err != nil {
		t.Fatalf("decodeAgentJSON error: %v", err)
	}
	if out.MasteryScore != 77 || out.FollowUp.Question != "Q" || len(out.Diagnosis.MissingParts) != 1 {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestDecodeAgentJSONFindsNestedObjectWithKeys(t *testing.T) {
	var out QuestionGeneratorOutput
	input := "{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"questions\\\":{\\\"1\\\":\\\"讲一下\\\"},\\\"suggestions\\\":[\\\"补例子\\\"]}\\n```\"}}]}"
	if err := decodeAgentJSON(input, &out, "questions"); err != nil {
		t.Fatalf("decodeAgentJSON error: %v", err)
	}
	if out.Questions["1"] == "" || len(out.Suggestions) != 1 {
		t.Fatalf("unexpected output: %+v", out)
	}
}
