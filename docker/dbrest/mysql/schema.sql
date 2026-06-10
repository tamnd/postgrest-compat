-- MySQL schema for the client compatibility test suite.
-- All tables live in one database (api); no role/grant statements (NativeRoles=false).
-- Engine-appropriate types:
--   text[]   → JSON               (stored as ["go","sql"] arrays)
--   boolean  → BOOL/TINYINT(1)   (coerced to Go bool by dbrest result layer)
--   serial   → INT AUTO_INCREMENT
--   UUID pk  → VARCHAR(36) DEFAULT (UUID())   (MySQL 8.0.13+)
--   FTS      → FULLTEXT INDEX on text columns (MATCH ... AGAINST IN BOOLEAN MODE)

CREATE TABLE IF NOT EXISTS todos (
    id   INT          AUTO_INCREMENT PRIMARY KEY,
    done BOOL         NOT NULL DEFAULT FALSE,
    task VARCHAR(500) NOT NULL,
    due  DATE         DEFAULT NULL,
    tags JSON         NOT NULL DEFAULT (JSON_ARRAY()),
    UNIQUE KEY uniq_task (task(255)),
    FULLTEXT KEY ft_task (task)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS persons (
    id    INT         AUTO_INCREMENT PRIMARY KEY,
    name  VARCHAR(255) NOT NULL,
    age   INT          DEFAULT NULL,
    email VARCHAR(255) UNIQUE DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS assignments (
    id        INT AUTO_INCREMENT PRIMARY KEY,
    person_id INT DEFAULT NULL,
    todo_id   INT DEFAULT NULL,
    UNIQUE KEY uniq_person_todo (person_id, todo_id),
    CONSTRAINT fk_assign_person FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE CASCADE,
    CONSTRAINT fk_assign_todo   FOREIGN KEY (todo_id)   REFERENCES todos(id)   ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS channels (
    id   INT          AUTO_INCREMENT PRIMARY KEY,
    slug VARCHAR(255) NOT NULL UNIQUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS messages (
    id         INT  AUTO_INCREMENT PRIMARY KEY,
    message    TEXT DEFAULT NULL,
    channel_id INT  DEFAULT NULL,
    person_id  INT  DEFAULT NULL,
    CONSTRAINT fk_msg_channel FOREIGN KEY (channel_id) REFERENCES channels(id),
    CONSTRAINT fk_msg_person  FOREIGN KEY (person_id)  REFERENCES persons(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS countries (
    id   INT          AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cities (
    id         INT          AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    country_id INT          DEFAULT NULL,
    CONSTRAINT fk_city_country FOREIGN KEY (country_id) REFERENCES countries(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS movies (
    id           VARCHAR(36)  PRIMARY KEY DEFAULT (UUID()),
    name         VARCHAR(255) NOT NULL,
    tags         JSON         NOT NULL DEFAULT (JSON_ARRAY()),
    release_date DATE         DEFAULT NULL,
    watched_at   DATETIME     DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS companies (
    id      INT          AUTO_INCREMENT PRIMARY KEY,
    name    VARCHAR(255) NOT NULL,
    address VARCHAR(255) DEFAULT NULL,
    phone   VARCHAR(50)  DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS people (
    id         INT          AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    age        INT          DEFAULT NULL,
    company_id INT          DEFAULT NULL,
    CONSTRAINT fk_people_company FOREIGN KEY (company_id) REFERENCES companies(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- items is accessible via Accept-Profile: private (same database, resolved via searchPath)
CREATE TABLE IF NOT EXISTS items (
    id   INT          AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
