package extractor

import (
	"strings"
	"testing"
)

const testHTML = `<!DOCTYPE html>
<html>
<head><title>Test Survey</title></head>
<body>
<form>
  <legend>What is the make and model of your vehicle?</legend>
  <input type="radio" name="q1" value="1"><label>Toyota</label>
  <input type="radio" name="q1" value="2"><label>Honda</label>
  <script>var surveyData = {"questionId": "q1", "type": "single"};</script>
</form>
</body>
</html>`

// TestExtractPageMetadata_PageIDExtraction 验证 page_id 清洗逻辑
func TestExtractPageMetadata_PageIDExtraction(t *testing.T) {
	meta, err := extractPageMetadata("s1", testHTML)
	if err != nil {
		t.Fatalf("extractPageMetadata: %v", err)
	}
	if !strings.Contains(meta.PageID, "what") {
		t.Errorf("unexpected PageID: %q", meta.PageID)
	}
}
