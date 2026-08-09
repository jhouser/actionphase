
-- Game loot tables
CREATE TABLE game_loot_tables (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Game loot tables
CREATE TABLE game_loot_table_contents (
    id SERIAL PRIMARY KEY,
    loot_table_id INTEGER NOT NULL REFERENCES game_loot_tables(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    data TEXT
);