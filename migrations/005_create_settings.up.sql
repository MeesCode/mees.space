CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_api_key', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_system_prompt', '');
