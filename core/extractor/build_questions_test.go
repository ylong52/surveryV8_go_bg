package extractor

import (
	"testing"

	"survey.cli/core/models"
)

// testInteraction 最小化 Interaction 辅助构造
func testInteraction(qType string) Interaction {
	return Interaction{
		QID:          models.QID{Value: ""},
		QuestionType: qType,
	}
}

// surveyHTML 含两个问题：q1=2选项(radio), q2=3选项(checkbox)
// 外层 <div class="survey"> 确保正则 (?:</div>|<div[^>]) 能匹配
const surveyHTML = `<html><body>
<div class="survey">
<div class="question single">
  <input type="radio" id="q1_1" name="q1" value="male"><label for="q1_1">Male</label>
  <input type="radio" id="q1_2" name="q1" value="female"><label for="q1_2">Female</label>
</div>
<div class="question multi">
  <input type="checkbox" id="q2_1" name="q2" value="tech"><label for="q2_1">Tech</label>
  <input type="checkbox" id="q2_2" name="q2" value="sports"><label for="q2_2">Sports</label>
  <input type="checkbox" id="q2_3" name="q2" value="music"><label for="q2_3">Music</label>
</div>
</div>
</body></html>`

// TestBuildQuestionsFromHTML_OptIDs — 검수 기준 핵심:
//   q1 opt_id = 11, 12
//   q2 opt_id = 21, 22, 23
func TestBuildQuestionsFromHTML_OptIDs(t *testing.T) {
	interactions := []Interaction{
		testInteraction("single"),
		testInteraction("multi"),
	}

	questions := BuildQuestionsFromHTML(surveyHTML, interactions)

	if len(questions) != 2 {
		t.Fatalf("questions 수 기대 2, 실제 %d", len(questions))
	}

	// q1 검증
	q1 := questions[0]
	if q1.QuestionID != "q1" {
		t.Errorf("q1.QuestionID = %q, want %q", q1.QuestionID, "q1")
	}
	if len(q1.Options) != 2 {
		t.Fatalf("q1 선택지 수 기대 2, 실제 %d", len(q1.Options))
	}
	wantQ1IDs := []int{11, 12}
	for i, opt := range q1.Options {
		if opt.OptID != wantQ1IDs[i] {
			t.Errorf("q1.Options[%d].OptID = %d, want %d", i, opt.OptID, wantQ1IDs[i])
		}
	}

	// q2 검증
	q2 := questions[1]
	if q2.QuestionID != "q2" {
		t.Errorf("q2.QuestionID = %q, want %q", q2.QuestionID, "q2")
	}
	if len(q2.Options) != 3 {
		t.Fatalf("q2 선택지 수 기대 3, 실제 %d", len(q2.Options))
	}
	wantQ2IDs := []int{21, 22, 23}
	for i, opt := range q2.Options {
		if opt.OptID != wantQ2IDs[i] {
			t.Errorf("q2.Options[%d].OptID = %d, want %d", i, opt.OptID, wantQ2IDs[i])
		}
	}
}

// TestBuildQuestionsFromHTML_QuestionIDs — question_id 자동 할당
func TestBuildQuestionsFromHTML_QuestionIDs(t *testing.T) {
	interactions := []Interaction{
		testInteraction("single"),
		testInteraction("multi"),
	}

	questions := BuildQuestionsFromHTML(surveyHTML, interactions)

	if len(questions) < 2 {
		t.Fatalf("questions 수 기대 2, 실제 %d", len(questions))
	}
	if questions[0].QuestionID != "q1" {
		t.Errorf("questions[0].QuestionID = %q, want q1", questions[0].QuestionID)
	}
	if questions[1].QuestionID != "q2" {
		t.Errorf("questions[1].QuestionID = %q, want q2", questions[1].QuestionID)
	}
}

// TestBuildQuestionsFromHTML_Empty — 빈 HTML 입력 시 nil 반환
func TestBuildQuestionsFromHTML_Empty(t *testing.T) {
	result := BuildQuestionsFromHTML("", nil)
	if result != nil {
		t.Errorf("빈 HTML 시 nil 기대, 실제 %v", result)
	}
}
