CREATE INDEX IF NOT EXISTS idx_movie_title ON movie USING GIN (to_tsvector('simple', title));
CREATE INDEX IF NOT EXISTS idx_movie_genres ON movie USING GIN (genres);