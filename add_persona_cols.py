import sqlite3

conn = sqlite3.connect('data/survey202605.db')
cursor = conn.cursor()

# Add missing columns to survey_persona
try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN persona_mode TEXT DEFAULT 'auto'")
    print("Added persona_mode")
except:
    print("persona_mode already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN age_range TEXT")
    print("Added age_range")
except:
    print("age_range already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN gender TEXT")
    print("Added gender")
except:
    print("gender already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN race TEXT")
    print("Added race")
except:
    print("race already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN occupation TEXT")
    print("Added occupation")
except:
    print("occupation already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN income INTEGER")
    print("Added income")
except:
    print("income already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN education TEXT")
    print("Added education")
except:
    print("education already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN has_children INTEGER DEFAULT 0")
    print("Added has_children")
except:
    print("has_children already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN marital_status TEXT")
    print("Added marital_status")
except:
    print("marital_status already exists")

try:
    cursor.execute("ALTER TABLE survey_persona ADD COLUMN option_mapping TEXT")
    print("Added option_mapping")
except:
    print("option_mapping already exists")

conn.commit()
print("Done")
conn.close()
