/* OAuth 客户端表 */
CREATE TABLE IF NOT EXISTS oauth_clients (
    id BINARY(16) PRIMARY KEY,
    name VARCHAR(32) NOT NULL,
    redirect_uri VARCHAR(512) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='OAuth 客户端表';

/* 用户表 */
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    login_id VARCHAR(16) NOT NULL UNIQUE,
    nickname VARCHAR(16) NOT NULL,
    -- 密码本身 16 字符以内，但以 argon2id hash 方式存
    password_hash VARCHAR(255) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户表' AUTO_INCREMENT = 10000;


/* EnLangMemo 同步表 */

CREATE TABLE IF NOT EXISTS sync_units (
    user_id BIGINT NOT NULL,
    usn BIGINT NOT NULL,

    entity_id BINARY(16) NOT NULL,
    entity_type TINYINT NOT NULL COMMENT '1=collection, 2=deck, 3=note_type, 4=note, 5=processing_note, 6=card, 7=review_log',
    op TINYINT NOT NULL COMMENT '1=UPSERT, 2=DELETE',

    updated_at BIGINT NOT NULL,

    PRIMARY KEY (user_id, entity_id),
    -- 用来确定有哪些实体类型需要拉取数据的辅助索引
    INDEX ix_sync_units_pull (user_id ASC, entity_type ASC, usn ASC),
    -- 用于运维清理非常早之前就删除的同步单元
    INDEX ix_sync_units_retention (op, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 同步单元索引表';

-- 一个用户只能有一个集合
CREATE TABLE IF NOT EXISTS collections (
    user_id BIGINT UNSIGNED NOT NULL,
    -- UUIDv7
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    sqlite_schema_version INT NOT NULL DEFAULT 0,
    last_sync_time BIGINT NOT NULL DEFAULT 0,
    sync_cursor_usn BIGINT NOT NULL DEFAULT 0,

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    config JSON NOT NULL,

    is_deleted TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 集合表';

CREATE TABLE IF NOT EXISTS decks (
    user_id BIGINT UNSIGNED NOT NULL,
    -- UUIDv7
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    name VARCHAR(32) NOT NULL,
    updated_at BIGINT NOT NULL,

    new_cards_per_day INT NOT NULL DEFAULT 20,
    new_learned_today INT NOT NULL DEFAULT 0,
    learned_today INT NOT NULL DEFAULT 0,
    reviewed_today INT NOT NULL DEFAULT 0,
    config JSON NOT NULL,

    is_deleted TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 牌组表';

CREATE TABLE IF NOT EXISTS note_types (
    user_id BIGINT UNSIGNED NOT NULL,
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    name VARCHAR(32) NOT NULL,
    preset_template_id INT NOT NULL DEFAULT 0,

    updated_at BIGINT NOT NULL,

    note_template JSON NOT NULL,

    is_deleted TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 笔记模板表';

CREATE TABLE IF NOT EXISTS notes (
    user_id BIGINT UNSIGNED NOT NULL,
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    note_type_id BINARY(16) NOT NULL,

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,

    sense_id INT UNSIGNED NULL,
    fields JSON NOT NULL,

    is_deleted TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 笔记表';

CREATE TABLE IF NOT EXISTS processing_notes (
    user_id BIGINT UNSIGNED NOT NULL,
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    note_type_id BINARY(16) NOT NULL,

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,

    sense_id INT UNSIGNED NULL,
    fields JSON NOT NULL,

    is_deleted TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 待加工笔记表';

CREATE TABLE IF NOT EXISTS cards (
    user_id BIGINT UNSIGNED NOT NULL,
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    note_id BINARY(16) NOT NULL,
    deck_id BINARY(16) NOT NULL,

    updated_at BIGINT NOT NULL,

    difficulty DOUBLE NOT NULL,
    stability DOUBLE NOT NULL,
    scheduled_days INT NOT NULL,

    due BIGINT NOT NULL,
    last_review BIGINT NULL,
    lapses INT NOT NULL,
    learning_steps INT NOT NULL,
    repetitions INT NOT NULL,
    state TINYINT NOT NULL,
    queue TINYINT NOT NULL,

    is_deleted TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 卡片表';

CREATE TABLE IF NOT EXISTS review_logs (
    user_id BIGINT UNSIGNED NOT NULL,
    id BINARY(16) NOT NULL,
    usn BIGINT NOT NULL,

    card_id BINARY(16) NOT NULL,

    review_time BIGINT NOT NULL,
    scheduled_days INT NOT NULL,

    rating TINYINT NOT NULL,
    difficulty DOUBLE NOT NULL,
    stability DOUBLE NOT NULL,

    learning_steps INT NOT NULL,
    state TINYINT NOT NULL,
    duration INT NOT NULL,

    PRIMARY KEY (user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='EnLangMemo 复习日志表';
