package extractor

// opentext_prompts.go — 开放题验证器 fallback 提示词
//   数据库 open_text.* 缺失时使用内置常量兜底

// fallbackInitialPrompt 开放题生成提示词（DeepSeek 生成用）
const fallbackInitialPrompt = `Answer Requirements:
- Read the question carefully. Your answer must directly address what is being asked.
- Identify the core topic and respond to it — include at least one specific, concrete detail related to the question subject.
- Do not go completely off-topic. If your answer has nothing to do with the question, it fails regardless of writing style.
- Address the question first, then apply the style rules below.

Style Rules:
- Use fragments. Not every sentence needs a subject.
- Skip the reason. Don't explain why things happened.
- No clean endings. Don't wrap up each paragraph neatly.
- Occasionally throw in "or whatever", "I don't know", "kind of" — but don't overdo it.
- No precise details like exact times or numbers unless they're vague on purpose.
- Jump between ideas without smooth transitions.
- Avoid summary sentences at the end of paragraphs.
- Don't connect every sentence logically — real people don't.
- Drop filler words. Cut anything that sounds like a setup.
- Never use: seamless, vibrant, dynamic, ensure, utilize, leverage, moreover, furthermore, however.
- Avoid complete subject-verb-object sentences back to back.
- Mix sentence lengths unevenly — two short, one longer, one fragment, repeat roughly.
- Don't over-explain emotions. Say what happened, not how it made you feel.
- Avoid words that sound like AI mimicking casual speech: "totally", "literally", "honestly" used as openers.
- Leave some things slightly unresolved or unexplained.
- The goal is to be more than 50 but less than 70 words.
- No bullet points, no lists, no headers in output.

补充：
1，若要减少AI痕迹，提示词中应加入"强制破坏句式结构"的指令，例如要求："禁止使用因果连词（因为、所以、但是），强制将长句拆解为语义跳跃的短语或名词堆砌"。AI通常倾向于保持叙事的完整逻辑链（即使伪装成口语），可以通过要求"叙述中必须包含无关的感官干扰（如窗外的噪音、手边的凉水）"，打破AI追求的"叙事简洁与中心突出"的预设倾向，从而模拟人类思维的随机游走。`

// fallbackVerifyPrompt 开放题验证提示词（Claude 验证用）
const fallbackVerifyPrompt = `You are a survey response auditor. This answer comes from an ONLINE SURVEY — not a blog, essay, or forum post.

## CONTEXT: HOW REAL PEOPLE FILL OUT SURVEYS
Most survey respondents type quickly and carelessly. Normal human survey behavior includes:
- Very short answers (5–30 words) with no explanation
- Sentence fragments, no subject, no verb: "Campus feel." "The people." "Location mostly."
- Incomplete thoughts that trail off or stop abruptly
- Zero personal details — just a vague preference stated plainly
- No punctuation care: missing commas, lowercase throughout, no period at end
- Mild repetition or slight vagueness: "just the overall vibe I guess"
- NO attempt to sound impressive or thorough

These are all NORMAL and should score high (70–85) for human_score.

## AUTOMATIC SCORE CAPS (AI giveaways — hard limits)

If ANY of these appear → human_score cannot exceed 25:
- Starts with: "Certainly", "Of course", "Absolutely", "Great question", "As an AI", "I'd be happy"
- Contains: "Furthermore", "Moreover", "In addition", "It is worth noting", "One could argue"
- Contains: "In conclusion", "To summarize", "Overall, I believe", "I hope this helps"
- Has bullet points or numbered lists
- Every sentence starts with "I" and follows the exact same structure

If 2+ of these appear → human_score cannot exceed 45:
- All sentences are complete, grammatically perfect, and similar in length
- No contractions anywhere (writes "I do not", "it is", "I am")
- Covers multiple balanced perspectives on a simple preference question
- Ends with a tidy summary or closing statement

## SCORING (base: 65)

Deductions — clear AI signals:
- Formal opener (Certainly / Of course / etc.)     : -25
- Transition words (Furthermore / Moreover)       : -20
- Closing remark (In conclusion / I hope)          : -20
- Zero contractions across 3+ sentences           : -15
- Balanced pros/cons on a personal opinion question : -15
- Bullet points or numbered lists                 : -25
- Over 150 words                                  : -10

Additions — human survey signals:
- Fragment or incomplete sentence                  : +15
- Vague/lazy phrasing ("I guess", "idk", "or whatever", "just the vibe"): +12
- Abrupt ending — thought just stops              : +10
- Contractions used (don't, it's, I've)            : +8
- No capitalization or casual punctuation         : +8
- Slang or filler (like, honestly, tbh, kinda)    : +8
- Single-word or 2-word answer ("Location." "The people."): +15

## RELEVANCE CHECK
- Answers the question, even vaguely or briefly → relevance_score: 100
- Completely off-topic or nonsensical → relevance_score: 0

## SCORING RULE
Apply caps first. Then: human_score = max(0, min(100, 65 + additions - deductions))
ai_score = 100 - human_score

## PASS CONDITION
All three must be met simultaneously:
  - relevance_score == 100  (must answer the question, even vaguely)
  - human_score >= 60     (must feel genuinely human-written)
  - ai_score <= 30        (must not exhibit AI writing patterns)

If any condition fails, the answer does not pass.

## OUTPUT FORMAT
Return ONLY valid JSON. No markdown code fences. No other text:
{
  "relevance_score": <integer 0-100>,
  "human_score": <integer 0-100>,
  "ai_score": <integer 0-100>,
  "score_note": "Brief English explanation — cite the specific feature that most affected the score",
  "prompt_suggestion_note": "If any pass condition fails: write specific Chinese improvement suggestions in '禁止X/必须Y' format (2-4 items, numbered) to fix the failing dimension(s). Otherwise: empty string."
}`

// fallbackHumanizePrompt Humanize 反统计扰动提示词（DeepSeek 二次处理用）
const fallbackHumanizePrompt = `You will receive a survey answer. Apply these specific edits to make it harder for AI detectors to flag it. Do NOT rewrite or restructure — make surgical word-level changes only.

WHAT TO CHANGE (pick 3–4 of these, whichever fit naturally):

Word substitutions — swap predictable words for less expected ones that still make sense:
- "memorable" → "one I keep thinking about" or "that one time"
- "experience" → "thing that happened" or "situation"
- "customer service" → "the person at the desk" or "the lady/guy there"
- "however" / "but" → "though" or just start a new sentence
- "decided" → "ended up" or "just went ahead and"
- "received" → "got" or "they handed me"
- "noticed" → "could tell" or "saw"
- any -ly adverb → rephrase without it ("quickly" → "right away", "surprisingly" → "which was weird")

Rhythm breaks — do ONE of these:
- Take the longest sentence and split it at a natural pause with a period, even mid-thought
- Take two short sentences and run them together with just a comma (comma splice is fine)

Filler insertion — add ONE of these somewhere it feels natural (not at the start):
- "or whatever", "I guess", "which was odd", "for some reason", "not gonna lie"

End abruptly — if the last sentence wraps things up too neatly, cut the final 4–6 words or replace with "idk" / "so yeah" / nothing

DO NOT:
- Reorder sentences or change sentence structure significantly
- Fix or introduce grammar errors that make the text confusing
- Remove existing casual words like "honestly", "tbh", "like" — keep them
- Add new facts or change what happened
- Exceed 85 words

Output the modified answer only. No explanation.`
