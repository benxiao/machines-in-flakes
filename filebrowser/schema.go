package main

import (
	"context"
)

const schema = `
CREATE TABLE IF NOT EXISTS indexed_paths (
	id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	path TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS video_positions (
	path         TEXT PRIMARY KEY,
	position_sec DOUBLE PRECISION NOT NULL DEFAULT 0,
	watch_count  BIGINT NOT NULL DEFAULT 0,
	updated_at   TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS playlists (
	id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name       TEXT NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS playlist_items (
	id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
	path        TEXT NOT NULL,
	UNIQUE (playlist_id, path)
);
CREATE TABLE IF NOT EXISTS playlist_state (
	playlist_id   BIGINT PRIMARY KEY REFERENCES playlists(id) ON DELETE CASCADE,
	current_index INT NOT NULL DEFAULT 0,
	position_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,
	updated_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS users (
	id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now()
);
ALTER TABLE indexed_paths ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT TRUE;
CREATE TABLE IF NOT EXISTS file_index (
	user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	path      TEXT NOT NULL,
	filename  TEXT NOT NULL,
	file_type TEXT NOT NULL,
	dir_path  TEXT NOT NULL,
	mtime     TIMESTAMPTZ,
	PRIMARY KEY (user_id, path)
);
ALTER TABLE file_index ADD COLUMN IF NOT EXISTS mtime TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS file_index_search ON file_index (user_id, lower(filename));
CREATE TABLE IF NOT EXISTS file_hashes (
	path      TEXT PRIMARY KEY,
	size      BIGINT NOT NULL,
	mtime     TIMESTAMPTZ NOT NULL,
	sha256    TEXT NOT NULL,
	hashed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS favorites (
	user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	path       TEXT NOT NULL,
	is_folder  BOOLEAN NOT NULL DEFAULT FALSE,
	position   INT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ DEFAULT now(),
	PRIMARY KEY (user_id, path)
);
CREATE TABLE IF NOT EXISTS trash_items (
	id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	original_path TEXT NOT NULL,
	trash_path    TEXT NOT NULL UNIQUE,
	is_folder     BOOLEAN NOT NULL,
	deleted_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
	deleted_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS folder_play_time (
	user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	media_type TEXT NOT NULL DEFAULT 'video',
	folder     TEXT NOT NULL,
	seconds    BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (user_id, media_type, folder)
);
CREATE TABLE IF NOT EXISTS track_bookmarks (
	id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	path         TEXT NOT NULL,
	label        TEXT NOT NULL,
	position_sec DOUBLE PRECISION NOT NULL,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS track_bookmarks_path ON track_bookmarks (user_id, path);
`

const migrations = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='indexed_paths' AND column_name='user_id') THEN
    ALTER TABLE indexed_paths ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE indexed_paths SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
    ALTER TABLE indexed_paths DROP CONSTRAINT IF EXISTS indexed_paths_path_key;
    ALTER TABLE indexed_paths ADD CONSTRAINT indexed_paths_user_id_path_key UNIQUE (user_id, path);
  END IF;
  DROP TABLE IF EXISTS settings;
  ALTER TABLE favorites ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0;
  UPDATE favorites f SET position = sub.rn - 1
    FROM (SELECT user_id, path, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at) AS rn FROM favorites) sub
    WHERE f.user_id = sub.user_id AND f.path = sub.path AND f.position = 0 AND f.created_at IS NOT NULL;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='playlists' AND column_name='user_id') THEN
    ALTER TABLE playlists ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE playlists SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='playlist_items' AND column_name='position') THEN
    ALTER TABLE playlist_items ADD COLUMN position INT NOT NULL DEFAULT 0;
    UPDATE playlist_items pi SET position = sub.rn - 1
    FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY playlist_id ORDER BY id) AS rn FROM playlist_items) sub
    WHERE pi.id = sub.id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='video_positions' AND column_name='user_id') THEN
    ALTER TABLE video_positions ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE video_positions SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
    IF NOT EXISTS (SELECT 1 FROM video_positions WHERE user_id IS NULL) THEN
      ALTER TABLE video_positions DROP CONSTRAINT IF EXISTS video_positions_pkey;
      ALTER TABLE video_positions ADD PRIMARY KEY (user_id, path);
    END IF;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_admin') THEN
    ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
    UPDATE users SET is_admin = TRUE WHERE id = (SELECT MIN(id) FROM users);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='path_grants') THEN
    CREATE TABLE path_grants (
      path_id BIGINT NOT NULL REFERENCES indexed_paths(id) ON DELETE CASCADE,
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      PRIMARY KEY (path_id, user_id)
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='play_time') THEN
    CREATE TABLE play_time (
      user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      day        DATE NOT NULL,
      media_type TEXT NOT NULL DEFAULT 'video',
      seconds    BIGINT NOT NULL DEFAULT 0,
      PRIMARY KEY (user_id, day, media_type)
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='play_time' AND column_name='media_type') THEN
    ALTER TABLE play_time ADD COLUMN media_type TEXT NOT NULL DEFAULT 'video';
    ALTER TABLE play_time DROP CONSTRAINT play_time_pkey;
    ALTER TABLE play_time ADD PRIMARY KEY (user_id, day, media_type);
  END IF;
END $$;
`

func (a *App) initSchema(ctx context.Context) error {
	if _, err := a.db.Exec(ctx, schema); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, migrations)
	return err
}
