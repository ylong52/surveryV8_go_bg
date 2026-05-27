# -*- coding: utf-8 -*-
"""
prompts 表种子数据生成器（SQLite 版）

从源码 YAML 配置文件中读取提示词内容，生成完整的 INSERT SQL 语句，
用于填充 prompts 表。

使用方式:
    python seed_prompts.py            # 输出 SQL 到 stdout
    python seed_prompts.py -o out.sql # 输出 SQL 到文件
    python seed_prompts.py --run       # 直接执行到 data/survey.db

数据库实际表结构（survey.db prompts 表）:
    id          INTEGER PRIMARY KEY AUTOINCREMENT  (自动生成，不能手动指定)
    prompt_key  TEXT NOT NULL
    prompt_type TEXT NOT NULL
    question_type TEXT
    content     TEXT NOT NULL
    description TEXT
    yaml_template TEXT
    version     INTEGER NOT NULL DEFAULT 1
    is_active   INTEGER NOT NULL DEFAULT 1
    created_at  TEXT NOT NULL DEFAULT (datetime('now','localtime'))
    updated_at  TEXT NOT NULL DEFAULT (datetime('now','localtime'))
    UNIQUE (prompt_key ASC)

数据来源:
    - content:       configs/prompts.yaml        (题型提示词、系统提示词等)
    - yaml_template: configs/yaml_templates.yaml  (30种题型的占位符模板)
    - yaml_structure: configs/yaml_structure_prompt.yaml (YAML结构提取专用)
    - yaml_prompts:  core/extractor/yaml_prompts.yaml    (YAML提取规则)
"""

import argparse
import os
import sys
from datetime import datetime

import yaml

# ---------------------------------------------------------------------------
# 路径配置
# ---------------------------------------------------------------------------
BASE_DIR = os.path.dirname(os.path.abspath(__file__))
# 项目根目录（cmd/initdb/ -> project_root/）
ROOT_DIR = os.path.dirname(os.path.dirname(BASE_DIR))
PROMPTS_YAML_PATH = os.path.join(ROOT_DIR, "configs", "prompts.yaml")
YAML_TEMPLATES_PATH = os.path.join(ROOT_DIR, "configs", "yaml_templates.yaml")
YAML_PROMPTS_PATH = os.path.join(ROOT_DIR, "core", "extractor", "yaml_prompts.yaml")
DB_PATH = os.path.join(ROOT_DIR, "data", "survey.db")

# ---------------------------------------------------------------------------
# SQL 转义（SQLite 标准：单引号转两个单引号）
# ---------------------------------------------------------------------------
def sql_escape(text: str) -> str:
    """将 Python 字符串转义为可安全嵌入 SQL 单引号字符串。"""
    if text is None:
        return "NULL"
    escaped = text.replace("'", "''")
    return f"'{escaped}'"


def sql_escape_or_null(text: str) -> str:
    """yaml_template 字段专用：空字符串转 NULL。"""
    if text is None or text.strip() == "":
        return "NULL"
    escaped = text.replace("'", "''")
    return f"'{escaped}'"


# ---------------------------------------------------------------------------
# YAML 加载
# ---------------------------------------------------------------------------
def load_yaml(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


# ---------------------------------------------------------------------------
# 第 1 类：题型提示词 (prompt_type = 'question_type')
# ---------------------------------------------------------------------------
# question_type（题型名）→ yaml_templates.yaml 中的模板 key
QUESTION_TO_TEMPLATE_KEY = {
    "single":          "single",
    "multiple":      "multi",      # yaml_templates.yaml 中 key 是 "multi"
    "select":          "select",
    "matrix_radio":    "matrix_radio",
    "matrix_checkbox": "matrix_checkbox",
    "email":          "email",
    "phone":          "phone",
    "number":         "number",
    "date":           "date",
    "time":           "time",
    "open_end":       "open_end",
    "open_text":      "open_text",
    "semantic_diff":  "semantic_diff",
    "slider":         "slider",
    "nps":            "nps",
    "likert":         "likert",
    "likert_card":    "likert_card",
    "multi_select":   "multi_select",
    "ranking_drag":   "ranking_drag",
    "ranking_select": "ranking_select",
    "max_diff":       "max_diff",
    "conjoint":       "conjoint",
    "cardsort_drag":  "cardsort_drag",
    "cardsort_select":"cardsort_select",
    "image_select":    "image_select",
    "heatmap":        "heatmap",
    "mixed":          "mixed",
    "unknown":        "unknown",
    "instruction":    "instruction",
}

# question_type → prompt_key
QUESTION_TYPE_KEY_MAP = {
    "single":          "qtype.single",
    "multiple":        "qtype.multiple",
    "select":          "qtype.select",
    "matrix_radio":    "qtype.matrix_radio",
    "matrix_checkbox": "qtype.matrix_checkbox",
    "email":          "qtype.email",
    "phone":          "qtype.phone",
    "number":         "qtype.number",
    "date":           "qtype.date",
    "time":           "qtype.time",
    "open_end":       "qtype.open_end",
    "open_text":      "qtype.open_text",
    "semantic_diff":  "qtype.semantic_diff",
    "slider":         "qtype.slider",
    "nps":            "qtype.nps",
    "likert":         "qtype.likert",
    "likert_card":    "qtype.likert_card",
    "multi_select":   "qtype.multi_select",
    "ranking_drag":   "qtype.ranking_drag",
    "ranking_select": "qtype.ranking_select",
    "max_diff":       "qtype.max_diff",
    "conjoint":       "qtype.conjoint",
    "cardsort_drag":  "qtype.cardsort_drag",
    "cardsort_select":"qtype.cardsort_select",
    "image_select":   "qtype.image_select",
    "heatmap":        "qtype.heatmap",
    "mixed":          "qtype.mixed",
    "unknown":        "qtype.unknown",
    "instruction":    "qtype.instruction",
}

QUESTION_TYPE_DESCRIPTIONS = {
    "single":          "单选题处理指令",
    "multiple":        "多选题处理指令",
    "select":          "下拉选择题处理指令",
    "matrix_radio":    "矩阵单选题处理指令",
    "matrix_checkbox": "矩阵多选题处理指令",
    "email":          "邮箱输入题处理指令",
    "phone":          "电话输入题处理指令",
    "number":         "数字输入题处理指令",
    "date":           "日期输入题处理指令",
    "time":           "时间输入题处理指令",
    "open_end":       "开放式长文本题处理指令",
    "open_text":      "开放式短文本追问题处理指令",
    "semantic_diff":  "语义差异量表题处理指令",
    "slider":         "滑动条题处理指令",
    "nps":            "NPS推荐意愿题处理指令",
    "likert":         "Likert量表矩阵题处理指令",
    "likert_card":    "卡片式量表/星级评分处理指令",
    "multi_select":   "选出最X题(multi_select)处理指令",
    "ranking_drag":   "拖拽排序题处理指令",
    "ranking_select": "选择排序题处理指令",
    "max_diff":       "MaxDiff/Best-Worst题处理指令",
    "conjoint":       "联合分析题处理指令",
    "cardsort_drag":  "拖拽分类题处理指令",
    "cardsort_select":"下拉分类题处理指令",
    "image_select":   "图片选择题处理指令",
    "heatmap":        "热图点击题处理指令",
    "mixed":          "混合题型/未知题型兜底处理指令",
    "unknown":        "无法识别题型降级兜底处理指令",
    "instruction":    "说明页/无交互元素页面处理指令",
}

# 兜底 content（prompts.yaml 中没有独立定义的题型）
FALLBACK_QUESTION_CONTENT = {
    "mixed": """== MIXED / UNKNOWN QUESTION TYPE ==

【识别特征】
- 同一页面包含多种题型，或 AI 无法明确归类时使用此类型
- 页面 HTML 结构复杂，包含多种输入元素

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，禁止改写
- showAnswer：必须使用 HTML 中的原始选项文本，禁止改写

【强制约束 — 禁止拆分！】
- 同一交互组的多个题型 = 一个 interaction
- 不要将不同题型拆分为多个 interaction

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 字符串或字符串数组（根据题型决定）

【推荐值生成规则】
- 根据各题型规则分别处理
- 遵循 core_rules 中的通用约束""",

    "unknown": """== UNKNOWN QUESTION TYPE ==

【识别特征】
- 无法从 HTML 识别出任何已知题型模式时使用此类型
- 页面 HTML 结构异常或使用了非标准控件

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中最可能的题目文本，禁止改写
- showAnswer：必须使用 HTML 中存在的选项文本，禁止凭空捏造

【强制约束 — 禁止凭空捏造！】
- showAnswer 必须是 HTML 中存在的值！

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 根据元素类型推断的合理答案

【推荐值生成规则】
- 优先尝试识别 input/select/textarea 等基本元素类型
- 使用通用逻辑推断答案格式""",

    "instruction": """== INSTRUCTION PAGE ==

【识别特征】
- 页面无任何交互元素（无 input/select/textarea/button 等）
- 仅包含说明文字、图片、标题等非输入内容
- 典型页面：问卷说明、隐私政策、答题指引、感谢页

【强制约束】
- 此类页面不生成任何 interaction
- 仅记录页面元数据

【页面属性】
- is_description_page: true
- has_next_button: 通常为 true
- has_submit_button: 通常为 false
- is_thank_you_page: 根据页面内容判断""",
}


def build_question_type_records(prompts_yaml: dict, yaml_templates: dict) -> list:
    """构建所有题型记录（prompt_type='question_type'）。"""
    records = []
    templates = yaml_templates.get("templates", {})

    qt_yaml = prompts_yaml.get("question_types", {})
    choice_yaml = qt_yaml.get("choice", {})
    matrix_yaml = qt_yaml.get("matrix", {})
    ranking_yaml = qt_yaml.get("ranking", {})
    cardsort_yaml = qt_yaml.get("cardsort", {})

    question_contents = {
        "single":          choice_yaml.get("single", ""),
        "multiple":        choice_yaml.get("multiple", ""),
        "select":          choice_yaml.get("select", ""),
        "matrix_radio":    matrix_yaml.get("matrix_radio", ""),
        "matrix_checkbox": matrix_yaml.get("matrix_checkbox", ""),
        "email":          qt_yaml.get("email", ""),
        "phone":          qt_yaml.get("phone", ""),
        "number":         qt_yaml.get("number", ""),
        "date":           qt_yaml.get("date", ""),
        "time":           qt_yaml.get("time", ""),
        "open_end":       qt_yaml.get("open_end", ""),
        "open_text":      qt_yaml.get("open_text", ""),
        "semantic_diff":  qt_yaml.get("semantic_diff", ""),
        "slider":         qt_yaml.get("slider", ""),
        "nps":            qt_yaml.get("nps", ""),
        "likert":         qt_yaml.get("likert", ""),
        "likert_card":    qt_yaml.get("likert_card", ""),
        "multi_select":   qt_yaml.get("multi_select", ""),
        "ranking_drag":   ranking_yaml.get("ranking_drag", ""),
        "ranking_select": ranking_yaml.get("ranking_select", ""),
        "max_diff":       qt_yaml.get("max_diff", ""),
        "conjoint":       qt_yaml.get("conjoint", ""),
        "cardsort_drag":  cardsort_yaml.get("cardsort_drag", ""),
        "cardsort_select":cardsort_yaml.get("cardsort_select", ""),
        "image_select":   qt_yaml.get("image_select", ""),
        "heatmap":        qt_yaml.get("heatmap", ""),
    }

    for qtype, prompt_key in QUESTION_TYPE_KEY_MAP.items():
        content = question_contents.get(qtype, "")
        if not content:
            content = FALLBACK_QUESTION_CONTENT.get(qtype, "")
        # yaml_templates.yaml 的 key 可能与 question_type 名不同（如 multiple→multi）
        template_key = QUESTION_TO_TEMPLATE_KEY.get(qtype, qtype)
        yaml_tpl = templates.get(template_key, "")
        description = QUESTION_TYPE_DESCRIPTIONS.get(qtype, "")

        records.append({
            "prompt_key": prompt_key,
            "prompt_type": "question_type",
            "question_type": qtype,
            "content": content,
            "description": description,
            "yaml_template": yaml_tpl,
            "version": 1,
            "is_active": 1,
        })

    return records


# ---------------------------------------------------------------------------
# 第 2 类：系统提示词 (prompt_type = 'system')
# ---------------------------------------------------------------------------
def build_system_records(prompts_yaml: dict) -> list:
    system_yaml = prompts_yaml.get("system", {})
    items = [
        ("system.default",            "默认系统提示词",           system_yaml.get("default", "")),
        ("system.analyze",           "分析模式系统提示词",       system_yaml.get("analyze", "")),
        ("system.structure",          "结构分析系统提示词",       system_yaml.get("structure", "")),
        ("system.role",               "HTML语义架构师角色提示词",  system_yaml.get("role", "")),
        ("system.conflict_resolution","冲突消解规则",             system_yaml.get("conflict_resolution", "")),
    ]
    return [
        {
            "prompt_key": pk,
            "prompt_type": "system",
            "question_type": None,
            "content": content,
            "description": desc,
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
        for pk, desc, content in items
    ]


# ---------------------------------------------------------------------------
# 第 3 类：答题策略 (prompt_type = 'strategy')
# ---------------------------------------------------------------------------
def build_strategy_records(prompts_yaml: dict) -> list:
    strategy_yaml = prompts_yaml.get("strategy", {})
    items = [
        ("strategy.conservative", "保守策略",      strategy_yaml.get("conservative", "")),
        ("strategy.aggressive",  "激进策略",       strategy_yaml.get("aggressive", "")),
        ("strategy.speed",      "速度优先策略",    strategy_yaml.get("speed", "")),
        ("strategy.authentic",  "真实策略",       strategy_yaml.get("authentic", "")),
        ("strategy.trap_aware", "陷阱检测策略",    strategy_yaml.get("trap_aware", "")),
    ]
    return [
        {
            "prompt_key": pk,
            "prompt_type": "strategy",
            "question_type": None,
            "content": content,
            "description": desc,
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
        for pk, desc, content in items
    ]


# ---------------------------------------------------------------------------
# 第 4 类：人设提示词 (prompt_type = 'persona')
# ---------------------------------------------------------------------------
def build_persona_records(prompts_yaml: dict) -> list:
    persona_yaml = prompts_yaml.get("persona", {})
    persona_rules = prompts_yaml.get("persona_rules", "")
    items = [
        ("persona.generic",      "通用人设（generic模式）",    PERSONA_GENERIC_CONTENT),
        ("persona.strict",      "严格答题者人设",              persona_yaml.get("strict", "")),
        ("persona.random",      "随机答题者人设",              persona_yaml.get("random", "")),
        ("persona.honest",      "诚实答题者人设",              persona_yaml.get("honest", "")),
        ("persona.persona_based","基于人设身份作答模板",        persona_yaml.get("persona_based", "")),
        ("persona.rules",       "基于人设的答题逻辑规则",       persona_rules),
    ]
    return [
        {
            "prompt_key": pk,
            "prompt_type": "persona",
            "question_type": None,
            "content": content,
            "description": desc,
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
        for pk, desc, content in items
    ]


# ---------------------------------------------------------------------------
# 第 5 类：核心规则 (prompt_type = 'core_rules')
# ---------------------------------------------------------------------------
def build_core_rules_record(prompts_yaml: dict) -> list:
    core_rules = prompts_yaml.get("core_rules", "")
    return [
        {
            "prompt_key": "core_rules",
            "prompt_type": "core_rules",
            "question_type": None,
            "content": core_rules,
            "description": "全局约束清单（所有题型共享）",
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
    ]


# ---------------------------------------------------------------------------
# 第 6 类：结构与格式 (prompt_type = 'structure')
# ---------------------------------------------------------------------------
def build_structure_records(prompts_yaml: dict) -> list:
    structure_yaml = prompts_yaml.get("structure", {})
    items = [
        ("structure.grouping",     "页面题型分组规则",    structure_yaml.get("grouping", "")),
        ("structure.analysis",     "页面结构分析（JSON分组）", structure_yaml.get("analysis", "")),
        ("structure.nav_buttons",  "导航按钮识别规则",    structure_yaml.get("nav_buttons", "")),
        ("structure.output_format", "最终JSON输出格式规范", structure_yaml.get("output_format", "")),
    ]
    return [
        {
            "prompt_key": pk,
            "prompt_type": "structure",
            "question_type": None,
            "content": content,
            "description": desc,
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
        for pk, desc, content in items
    ]


# ---------------------------------------------------------------------------
# 第 7 类：YAML 结构提取提示词 (prompt_type = 'yaml_structure')
# ---------------------------------------------------------------------------
YAML_STRUCTURE_SYSTEM = """你是一个问卷结构解析助手。我会给你一段海外调查问卷的 HTML 源码文件。

你的任务是从 HTML 中提取题目结构，生成标准化的 YAML 模板。
所有动态内容必须使用占位符 `<<<FROM_HTML:xxx>>>` 格式，禁止输出实际数据值。

【占位符格式】
- q1_title → "<<<FROM_HTML:q1_title>>>"
- q1_type → "<<<FROM_HTML:q1_type>>>"
- q1_required → "<<<FROM_HTML:q1_required>>>"
- q1_opt1 / q1_opt2 → "<<<FROM_HTML:q1_opt1>>>" / "<<<FROM_HTML:q1_opt2>>>" 等
- q1_col1 / q1_col2 → "<<<FROM_HTML:q1_col1>>>" / "<<<FROM_HTML:q1_col2>>>" 等
- q1_row1 / q1_row2 → "<<<FROM_HTML:q1_row1>>>" / "<<<FROM_HTML:q1_row2>>>" 等
- q1_placeholder → "<<<FROM_HTML:q1_placeholder>>>"
- q1_response_type → "<<<FROM_HTML:q1_response_type>>>"
- q1_orientation → "<<<FROM_HTML:q1_orientation>>>"
- q1_option_sort → "<<<FROM_HTML:q1_option_sort>>>"
- has_next_button / has_submit_button → "<<<FROM_HTML:has_next_button>>>" / "<<<FROM_HTML:has_submit_button>>>"
- is_thank_you_page / is_description_page → "<<<FROM_HTML:is_thank_you_page>>>" / "<<<FROM_HTML:is_description_page>>>"

【输出格式】
直接输出 YAML，不要 markdown 代码块包裹。

【严格禁止 - 必须遵守】
1. 绝对禁止输出 `fingerprint` 字段！
2. 绝对禁止输出任何实际文本值！所有内容必须用占位符！
3. 不要使用 markdown 代码块包裹 YAML
4. 不要遗漏 page_metadata 部分
5. 不要遗漏 response_format 部分"""

YAML_STRUCTURE_USER_PREFIX = """---

**任务说明**

请从以下 HTML 中提取题目结构，为每道题生成对应的 YAML 占位符模板。

**页面元数据识别**
- has_next_button: 从 HTML 中查找 Next/Continue/Proceed 按钮
- has_submit_button: 从 HTML 中查找 Submit 按钮
- is_thank_you_page: 从 HTML 中查找感谢页面特征
- is_description_page: 从 HTML 中查找说明页特征

**题型识别规则**
- radio / single → single
- checkbox / multi → multi
- table + radio → matrix_radio
- table + checkbox → matrix_checkbox
- likert 量表 → likert
- 语义量表 → semantic_diff
- slider → slider
- nps → nps
- rating / likert_card → likert_card
- select → select
- input[type=text] → open_text
- textarea → open_end
- input[type=email] → email
- input[type=tel] → phone
- input[type=number] → number
- input[type=date] → date
- input[type=time] → time
- image_select → image_select
- ranking_drag → ranking_drag
- ranking_select → ranking_select
- multi_select（含 xnumber）→ multi_select
- max_diff → max_diff
- conjoint → conjoint
- cardsort_drag → cardsort_drag
- cardsort_select → cardsort_select
- heatmap → heatmap
- instruction / NAV_BUTTON → instruction
- mixed → mixed
- 无法识别 → unknown

**输出要求**
1. 输出完整的 survey_page 结构（包含 page_metadata）
2. 每道题必须包含 response_format
3. 所有动态值使用占位符
4. 直接输出 YAML，不要 markdown 代码块包裹

HTML 源码如下："""

YAML_ANSWER_SYSTEM = """你是一个问卷答题助手，专门根据问卷题目内容和人设信息生成合理的推荐答案。

你的职责：
- 理解题目语义和选项
- 结合人设信息生成符合该身份的回答
- 确保 showAnswer 必须是 HTML 中存在的选项
- 禁止凭空捏造选项
- 禁止输出 YAML 结构，只输出推荐答案

**【极其重要】输出要求：**
- 只输出 interactions 数组和 page_summary
- 不要输出 survey_page、fingerprint 或任何 YAML 格式内容
- 直接输出 JSON，不要 markdown 代码块包裹

**输出格式必须是合法的 JSON**：
{
  "interactions": [
    {"questionType": "matrix_radio", "subject": "ABC（Never）", "showAnswer": "Never"}
  ],
  "page_summary": "This page surveys TV watching habits."
}"""

YAML_ANSWER_USER_PREFIX = """---

**【极其重要】你的任务是生成推荐答案，不是输出 YAML 结构！**

请仔细阅读以下信息，为每道题目生成推荐答案。

**【重要】数据来源优先级：HTML > YAML 占位符**

请严格按照以下优先级获取数据：
1. **HTML 内容（最高优先级）**：所有题目文本、rows、columns 必须从 HTML 中提取
2. **YAML 模板（仅格式参考）**：用于了解题型，不要复制到输出中
3. **禁止使用 YAML 中的占位符文本作为答案！**

**【任务说明】**

请根据以下信息为每道题目生成推荐答案。

**【通用答案生成规则】**

1. subject（语义主语）：
   - 单选/多选: 使用 HTML 中的题目文本
   - 矩阵: "{HTML中的行文本}（{HTML中的列文本}）"

2. showAnswer（推荐答案）：
   - 单选题: 字符串（如 "Female"）
   - 多选题: 字符串数组（如 ["Apple", "Samsung"]）
   - 矩阵题: 字符串（如 "Never"）

3. 矩阵题强制约束：
   - 每行输出一个 subject
   - 行与行之间推荐不同选项

**【输出格式 - 必须严格遵循】**

直接输出以下 JSON 结构，不要任何其他内容：
{
  "interactions": [
    {
      "questionType": "<<<FROM_HTML:q1_type>>>",
      "subject": "<<<FROM_HTML:q1_row1>>>（<<<FROM_HTML:q1_col1>>>）",
      "showAnswer": "<<<FROM_HTML:q1_col1>>>"
    }
  ],
  "page_summary": "<<<FROM_HTML:page_summary>>>"
}

**【重要约束】**
1. 只输出 interactions 数组，不要包含 survey_page 或其他 YAML 结构
2. 不要输出任何 YAML 格式的内容
3. page_summary 必须是英文句子

---

**YAML 结构（仅格式参考，禁止复制到输出中）**"""

YAML_TEMPLATE_TO_YAML_SYSTEM = """你是一个问卷数据生成助手。

你的职责：
- 根据提供的 YAML 模板结构（包含占位符）
- 结合当前 HTML 内容中的实际数据
- 生成包含最新数据的完整 YAML

重要规则：
- 严格保持模板的结构格式不变
- 用 HTML 中的实际题目文本、选项等数据替换占位符
- 不得遗漏任何题目
- 不得改变题目的类型和顺序

【重要】字符编码约束：
- 只使用 ASCII 标准字符输出 YAML
- 所有引号统一使用双引号 "
- 禁止使用 Unicode 弯引号

输出格式：
- 直接输出 YAML，不要任何解释文字
- 不要 markdown 代码块包裹"""

YAML_TEMPLATE_TO_YAML_USER = """---

**任务说明**

请根据以下 YAML 模板结构（包含占位符），结合当前 HTML 内容，生成最新的 YAML 数据。

---

**YAML 模板结构（参考格式）**

{yamlTemplate}

---

**当前 HTML 内容**

{html}

---

**生成要求**

1. 严格按照模板的结构格式输出
2. 用 HTML 中的实际数据替换占位符
3. 确保所有题目都被正确提取
4. 保持题目的原始顺序
5. 不得凭空添加或删除任何题目

直接输出 YAML 数据："""


# persona.generic 的 content（从 prompts.yaml persona_generic 配置提炼）
PERSONA_GENERIC_CONTENT = """【通用人设（generic 模式）】
不载人设，AI 以默认中立身份作答。
可选在 persona_generic 节点中定义通用人设，适合通用需求，不关心特定身份。

【预设人设配置】（prompts.yaml persona_generic）
- 年龄：38-45 岁（随机）
- 性别：Male/Female（随机）
- 种族：White/Caucasian / Black/African-American / Asian/Asian-American / Hispanic/Latino
- 职业：Operations Manager
- 收入：$95,000/年
- 学历：Bachelor's
- 家庭状态：已婚，有 16-18 岁正在申请大学的孩子
- 答题策略：优先家庭导向选项，关注子女教育和家庭消费
- 价格敏感度：中等，收入属于中等水平，家庭消费意愿较强，不极端追求低价

【选项映射】（人设配置中的 option_mapping）
- role_in_decision: "I make the decisions jointly with my spouse/partner"
- household_income: "$75,000 - $99,999"
- has_children: "Yes, I have a child/children aged 16-18"
- life_situation: "Help a student(s) in your household apply to college"
- major_purchase: "Make a large purchase (such as a car or home)"
- vacation: "Go on a vacation"

【答题逻辑规则】
- 收入相关：注重性价比，平衡品质与价格
- 年龄相关（36-50岁）：关注子女教育、职业稳定、退休储蓄
- 家庭状况：有家室有孩子，优先家庭保险、教育储蓄、家庭安全"""


def build_yaml_structure_records() -> list:
    items = [
        ("yaml_structure.system",                   "YAML结构提取：系统提示词（STRUCTURE_SYSTEM_PROMPT）",              YAML_STRUCTURE_SYSTEM),
        ("yaml_structure.user_prefix",             "YAML结构提取：用户提示前缀（STRUCTURE_USER_PROMPT_PREFIX）",      YAML_STRUCTURE_USER_PREFIX),
        ("yaml_structure.answer_system",           "YAML答案推荐：系统提示词（ANSWER_SYSTEM_PROMPT）",                YAML_ANSWER_SYSTEM),
        ("yaml_structure.answer_user_prefix",      "YAML答案推荐：用户提示前缀（ANSWER_USER_PROMPT_PREFIX）",         YAML_ANSWER_USER_PREFIX),
        ("yaml_structure.template_to_yaml_system",  "模板转YAML：系统提示词（TEMPLATE_TO_YAML_SYSTEM_PROMPT）",       YAML_TEMPLATE_TO_YAML_SYSTEM),
        ("yaml_structure.template_to_yaml_user",    "模板转YAML：用户提示（TEMPLATE_TO_YAML_USER_PROMPT）",          YAML_TEMPLATE_TO_YAML_USER),
    ]
    return [
        {
            "prompt_key": pk,
            "prompt_type": "yaml_structure",
            "question_type": None,
            "content": content,
            "description": desc,
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
        for pk, desc, content in items
    ]


# ---------------------------------------------------------------------------
# 第 8 类：YAML 提取专用提示词 (prompt_type = 'yaml_prompts')
# ---------------------------------------------------------------------------
def build_yaml_prompts_records() -> list:
    if not os.path.exists(YAML_PROMPTS_PATH):
        return []
    yaml_prompts = load_yaml(YAML_PROMPTS_PATH)
    items = [
        ("yaml_prompts.system_prefix",               "YAML提取：系统前缀",                    yaml_prompts.get("system_prefix", "")),
        ("yaml_prompts.multi_question_rules",        "YAML提取：多题页面处理规则",            yaml_prompts.get("multi_question_rules", "")),
        ("yaml_prompts.question_type_rules",         "YAML提取：32种题型识别规则",            yaml_prompts.get("question_type_rules", "")),
        ("yaml_prompts.field_extraction_rules",      "YAML提取：字段提取规则",                yaml_prompts.get("field_extraction_rules", "")),
        ("yaml_prompts.question_type_differential",   "YAML提取：题型差异化输出",              yaml_prompts.get("question_type_differential", "")),
        ("yaml_prompts.response_format_rules",       "YAML提取：response_format字段规则",     yaml_prompts.get("response_format_rules", "")),
        ("yaml_prompts.prohibitions",                "YAML提取：严格禁止行为",                yaml_prompts.get("prohibitions", "")),
        ("yaml_prompts.output_format_rules",          "YAML提取：输出格式要求",               yaml_prompts.get("output_format_rules", "")),
        ("yaml_prompts.page_detection_rules",        "YAML提取：页面属性检测规则",           yaml_prompts.get("page_detection_rules", "")),
    ]
    return [
        {
            "prompt_key": pk,
            "prompt_type": "yaml_prompts",
            "question_type": None,
            "content": content,
            "description": desc,
            "yaml_template": None,
            "version": 1,
            "is_active": 1,
        }
        for pk, desc, content in items
    ]


# ---------------------------------------------------------------------------
# SQL 生成（适配实际表结构，无 id 手动指定，使用 UPSERT）
# ---------------------------------------------------------------------------
# 表字段（无 id 手动指定，无 source_file）
TABLE_COLUMNS = [
    "prompt_key", "prompt_type", "question_type", "content",
    "description", "yaml_template", "version", "is_active",
]


def generate_upsert_sql(rec: dict) -> str:
    """生成 SQLite UPSERT SQL（INSERT OR REPLACE）。"""
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    def col(name: str) -> str:
        val = rec.get(name)
        if name == "question_type":
            return sql_escape(val) if val is not None else "NULL"
        if name == "yaml_template":
            return sql_escape_or_null(val or "")
        if name in ("version", "is_active"):
            return str(val)
        return sql_escape(val or "")

    cols = ", ".join(f'"{c}"' for c in TABLE_COLUMNS)
    vals = ", ".join(col(c) for c in TABLE_COLUMNS)

    return (
        f"INSERT OR REPLACE INTO prompts ({cols}, created_at, updated_at) "
        f"VALUES ({vals}, '{now}', '{now}');"
    )


# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------
def main():
    parser = argparse.ArgumentParser(description="生成 prompts 表种子数据 SQL（SQLite UPSERT）")
    parser.add_argument("-o", "--output", help="输出文件路径（默认 stdout）")
    parser.add_argument("--run", action="store_true", help="直接执行到 data/survey.db")
    parser.add_argument("--dry-run", action="store_true", help="仅打印记录数和分组")
    args = parser.parse_args()

    # 加载 YAML
    prompts_yaml = load_yaml(PROMPTS_YAML_PATH)
    yaml_templates = load_yaml(YAML_TEMPLATES_PATH)

    # 收集所有记录
    all_records = []
    all_records += build_question_type_records(prompts_yaml, yaml_templates)
    all_records += build_system_records(prompts_yaml)
    all_records += build_strategy_records(prompts_yaml)
    all_records += build_persona_records(prompts_yaml)
    all_records += build_core_rules_record(prompts_yaml)
    all_records += build_structure_records(prompts_yaml)
    all_records += build_yaml_structure_records()
    all_records += build_yaml_prompts_records()

    if args.dry_run:
        from collections import defaultdict
        groups = defaultdict(list)
        for r in all_records:
            groups[r["prompt_type"]].append(r["prompt_key"])
        print(f"总记录数: {len(all_records)}\n")
        for ptype, keys in sorted(groups.items()):
            print(f"[{ptype}] ({len(keys)} 条)")
            for k in sorted(keys):
                print(f"  - {k}")
            print()
        return

    # 生成 SQL
    lines = []
    lines.append("-- ============================================================")
    lines.append("-- prompts 表完整种子数据（UPSERT 模式）")
    lines.append(f"-- 生成时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    lines.append(f"-- 总记录数: {len(all_records)}")
    lines.append("-- ============================================================")
    lines.append("")
    lines.append("-- 使用 INSERT OR REPLACE：按 prompt_key 唯一键 upsert")
    lines.append("-- 已存在记录会被替换，不存在则插入")
    lines.append("")

    for rec in all_records:
        lines.append(generate_upsert_sql(rec))

    sql_text = "\n".join(lines)

    if args.run:
        import sqlite3
        conn = sqlite3.connect(DB_PATH)
        conn.executescript(sql_text)
        conn.commit()
        cur = conn.execute("SELECT COUNT(*) FROM prompts")
        count = cur.fetchone()[0]
        conn.close()
        print(f"SQL 已执行到: {DB_PATH}")
        print(f"prompts 表总记录数: {count}")
    elif args.output:
        with open(args.output, "w", encoding="utf-8") as f:
            f.write(sql_text)
        print(f"SQL 已写入: {args.output}")
        print(f"总记录数: {len(all_records)}")
    else:
        print(sql_text)


if __name__ == "__main__":
    main()
