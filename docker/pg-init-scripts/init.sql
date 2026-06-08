-- 数据库 PG
CREATE TABLE IF NOT EXISTS oauth_clients (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    redirect_uri text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
