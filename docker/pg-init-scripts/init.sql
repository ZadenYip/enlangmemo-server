-- 数据库 PG

CREATE TABLE IF NOT EXISTS oauth_clients (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    redirect_uri text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name varying(16) NOT NULL UNIQUE,
    -- 密码本身32字符以内，但以 argon2id hash 方式存
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
