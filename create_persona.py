import sqlite3

conn = sqlite3.connect('data/survey202605.db')
cursor = conn.cursor()

# Create survey_persona table
cursor.execute('''
CREATE TABLE IF NOT EXISTS survey_persona (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    survey_id TEXT NOT NULL,
    persona_key TEXT NOT NULL,
    description TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT (datetime('now', 'localtime')),
    updated_at DATETIME DEFAULT (datetime('now', 'localtime')),
    UNIQUE(survey_id, persona_key)
)
''')

# Insert default persona
cursor.execute('''
INSERT OR IGNORE INTO survey_persona (survey_id, persona_key, description, is_active)
VALUES ('__default__', 'default', 'A typical survey respondent answering questions honestly.', 1)
''')

conn.commit()
print('Tables created and default persona inserted')
conn.close()
