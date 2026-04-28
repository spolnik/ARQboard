-- +goose Up
CREATE TABLE users (
    id text PRIMARY KEY NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    is_admin integer NOT NULL DEFAULT 0,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE sessions (
    id text PRIMARY KEY NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at text NOT NULL,
    revoked_at text,
    created_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE workspaces (
    id text PRIMARY KEY NOT NULL,
    name text NOT NULL,
    slug text NOT NULL UNIQUE,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE workspace_members (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at text NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE boards (
    id text PRIMARY KEY NOT NULL,
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    slug text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX boards_workspace_id_idx ON boards(workspace_id);

CREATE TABLE board_members (
    board_id text NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at text NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (board_id, user_id)
);

CREATE TABLE columns (
    id text PRIMARY KEY NOT NULL,
    board_id text NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name text NOT NULL,
    position integer NOT NULL,
    wip_limit integer CHECK (wip_limit IS NULL OR wip_limit > 0),
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (board_id, position)
);

CREATE INDEX columns_board_id_idx ON columns(board_id);

CREATE TABLE cards (
    id text PRIMARY KEY NOT NULL,
    board_id text NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id text NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
    assignee_id text REFERENCES users(id) ON DELETE SET NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    position integer NOT NULL,
    due_at text,
    created_by text REFERENCES users(id) ON DELETE SET NULL,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX cards_board_id_idx ON cards(board_id);
CREATE INDEX cards_column_position_idx ON cards(column_id, position);
CREATE INDEX cards_assignee_id_idx ON cards(assignee_id);

CREATE TABLE card_comments (
    id text PRIMARY KEY NOT NULL,
    card_id text NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    author_id text REFERENCES users(id) ON DELETE SET NULL,
    body text NOT NULL,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX card_comments_card_id_idx ON card_comments(card_id);

CREATE TABLE activity_events (
    id text PRIMARY KEY NOT NULL,
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    board_id text REFERENCES boards(id) ON DELETE CASCADE,
    card_id text REFERENCES cards(id) ON DELETE CASCADE,
    actor_id text REFERENCES users(id) ON DELETE SET NULL,
    event_type text NOT NULL,
    payload text NOT NULL DEFAULT '{}',
    created_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX activity_events_workspace_created_idx ON activity_events(workspace_id, created_at DESC);
CREATE INDEX activity_events_card_created_idx ON activity_events(card_id, created_at DESC);

CREATE TABLE wiki_pages (
    id text PRIMARY KEY NOT NULL,
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    board_id text REFERENCES boards(id) ON DELETE CASCADE,
    title text NOT NULL,
    slug text NOT NULL,
    body_markdown text NOT NULL DEFAULT '',
    created_by text REFERENCES users(id) ON DELETE SET NULL,
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX wiki_pages_workspace_id_idx ON wiki_pages(workspace_id);
CREATE INDEX wiki_pages_board_id_idx ON wiki_pages(board_id);

-- +goose Down
DROP TABLE IF EXISTS wiki_pages;
DROP TABLE IF EXISTS activity_events;
DROP TABLE IF EXISTS card_comments;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS columns;
DROP TABLE IF EXISTS board_members;
DROP TABLE IF EXISTS boards;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
