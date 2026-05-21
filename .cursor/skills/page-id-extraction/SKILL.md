---
name: page-id-extraction
description: 从问卷 HTML 页面中提取并清洗 page_id。适用于分析问卷页面、提取页面指纹或操作 HTML 页面唯一标识符。page_id 用于缓存和页面识别。
---

# Page ID Extraction

Extract and clean `page_id` from survey HTML pages.

## Extraction Priority

Extract from HTML in this order (stop at first non-empty match):

1. `<legend>...</legend>` content (highest priority)
2. `<label>...</label>` content
3. `<div>...</div>` content (lowest priority)

## Cleaning Steps

Apply these steps in order:

| Step | Action | Example |
|------|--------|---------|
| 1 | Strip HTML tags | `<b>7.</b>` → `7.` |
| 2 | Decode HTML entities | `&nbsp;` → space, `&amp;` → `&` |
| 3 | Lowercase | `NOW` → `now` |
| 4 | Remove punctuation | `. , ; : ! ? ( ) [ ] { } ' "` deleted |
| 5 | Hyphen to space | `-` → space |
| 6 | Collapse spaces | `  ` → ` ` |
| 7 | Trim whitespace | `" text "` → `"text"` |
| 8 | Truncate to 100 chars | excess removed |

## Examples

| Raw Input | Cleaned page_id |
|-----------|-----------------|
| `7. Now, in which state...` | `7 now in which state` |
| `21. How familiar are you...` | `21 how familiar are you` |
| `What's your preference?` | `whats your preference` |

## Code Reference

```go
// Extraction patterns (priority order)
patterns := []string{
    `<legend[^>]*>([\s\S]*?)</legend>`,
    `<label[^>]*>([\s\S]*?)</label>`,
    `<div[^>]*>([\s\S]*?)</div>`,
}

// Cleaning function
func CleanPageID(raw string) string {
    s := StripTags(raw)                        // Step 1
    s = html.UnescapeString(s)                 // Step 2
    s = strings.ToLower(s)                     // Step 3
    s = regexpReplace(`[.,;:!?()\[\]{}'"]`)   // Step 4
    s = strings.ReplaceAll(s, "-", " ")        // Step 5
    s = regexpReplace(`\s+`, " ")              // Step 6
    s = strings.TrimSpace(s)                  // Step 7
    s = truncate(s, 100)                      // Step 8
    return s
}
```

## Storage Format

Cached pages use: `cache/<survey_id>/<page_id>.html`

## Notes

- Returns error if no pattern matches
- Max length: 100 characters (truncated, trailing space removed)
- Only hyphen `-` becomes space among punctuation
- Soft hyphen (U+00AD) is deleted
