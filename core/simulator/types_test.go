package simulator

import "testing"

// TestGetQuestionNumber — question_id 에서 번호 추출
func TestGetQuestionNumber(t *testing.T) {
	cases := []struct {
		qid  string
		want int
	}{
		{"q1", 1},
		{"q2", 2},
		{"q10", 10},
		{"q99", 99},
		{"Q3", 3},  // case-insensitive
		{"q0", 0},
		{"bad", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := GetQuestionNumber(c.qid)
		if got != c.want {
			t.Errorf("GetQuestionNumber(%q) = %d, want %d", c.qid, got, c.want)
		}
	}
}

// TestGetOptIDPrefix — opt_id 앞자리 계산
func TestGetOptIDPrefix(t *testing.T) {
	cases := []struct {
		qid  string
		want int
	}{
		{"q1", 10},
		{"q2", 20},
		{"q3", 30},
		{"q9", 90},
		{"q10", 100},
		{"q99", 990},
	}
	for _, c := range cases {
		got := GetOptIDPrefix(c.qid)
		if got != c.want {
			t.Errorf("GetOptIDPrefix(%q) = %d, want %d", c.qid, got, c.want)
		}
	}
}

// TestBuildOptID — 검수 기준 핵심:
//   q1 의 선택지 opt_id 는 11, 12, 13...
//   q2 의 선택지 opt_id 는 21, 22, 23...
func TestBuildOptID(t *testing.T) {
	cases := []struct {
		qid   string
		index int
		want  int
	}{
		// q1 시리즈
		{"q1", 1, 11},
		{"q1", 2, 12},
		{"q1", 3, 13},
		{"q1", 4, 14},
		// q2 시리즈
		{"q2", 1, 21},
		{"q2", 2, 22},
		{"q2", 3, 23},
		// q10 시리즈
		{"q10", 1, 101},
		{"q10", 2, 102},
	}
	for _, c := range cases {
		got := BuildOptID(c.qid, c.index)
		if got != c.want {
			t.Errorf("BuildOptID(%q, %d) = %d, want %d", c.qid, c.index, got, c.want)
		}
	}
}

// TestParseOptID — opt_id 에서 인덱스 역산
func TestParseOptID(t *testing.T) {
	cases := []struct {
		optID int
		want  int
	}{
		{11, 1},
		{12, 2},
		{13, 3},
		{21, 1},
		{22, 2},
		{23, 3},
		{101, 1},
	}
	for _, c := range cases {
		got := ParseOptID(c.optID)
		if got != c.want {
			t.Errorf("ParseOptID(%d) = %d, want %d", c.optID, got, c.want)
		}
	}
}
