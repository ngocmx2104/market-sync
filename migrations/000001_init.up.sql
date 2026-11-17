-- Initial schema (draft: review every index against the queries in docs/queries.md).

CREATE TABLE orders (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id         VARCHAR(64)     NOT NULL,
    source            VARCHAR(32)     NOT NULL,
    external_id       VARCHAR(64)     NOT NULL,
    status            VARCHAR(32)     NOT NULL,
    is_open           TINYINT(1) AS (status NOT IN ('completed', 'cancelled', 'returned') AND deleted_at IS NULL) STORED,
    total_amount      BIGINT          NOT NULL,
    currency          CHAR(3)         NOT NULL,
    source_created_at DATETIME(3)     NOT NULL,
    source_updated_at DATETIME(3)     NOT NULL,
    deleted_at        DATETIME(3)     NULL,
    raw               JSON            NULL,
    created_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_orders_key (tenant_id, source, external_id),
    KEY idx_orders_open (tenant_id, source, is_open, external_id),
    KEY idx_orders_tenant_updated (tenant_id, source_updated_at, id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE sync_runs (
    id           CHAR(16)     NOT NULL,
    tenant_id    VARCHAR(64)  NOT NULL,
    source       VARCHAR(32)  NOT NULL,
    resource     VARCHAR(32)  NOT NULL,
    window_from  DATETIME(3)  NOT NULL,
    window_to    DATETIME(3)  NOT NULL,
    status       VARCHAR(16)  NOT NULL,
    pages        INT          NOT NULL DEFAULT 0,
    records      INT          NOT NULL DEFAULT 0,
    quarantined  INT          NOT NULL DEFAULT 0,
    rejected     INT          NOT NULL DEFAULT 0,
    error_detail JSON         NULL,
    started_at   DATETIME(3)  NOT NULL,
    finished_at  DATETIME(3)  NULL,
    PRIMARY KEY (id),
    KEY idx_runs_tenant_started (tenant_id, started_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE quarantine (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id     VARCHAR(64)  NOT NULL,
    source        VARCHAR(32)  NOT NULL,
    external_id   VARCHAR(64)  NOT NULL,
    error_detail  JSON         NOT NULL,
    attempts      INT          NOT NULL DEFAULT 1,
    first_seen_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    last_seen_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    resolved_at   DATETIME(3)  NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_quarantine_key (tenant_id, source, external_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE rejections (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id    VARCHAR(64)  NOT NULL,
    source       VARCHAR(32)  NOT NULL,
    external_id  VARCHAR(64)  NOT NULL,
    raw          JSON         NULL,
    error_detail JSON         NOT NULL,
    created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_rejections_tenant_created (tenant_id, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE reconcile_reports (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id   VARCHAR(64)  NOT NULL,
    source      VARCHAR(32)  NOT NULL,
    checked     INT          NOT NULL,
    refreshed   INT          NOT NULL,
    deleted     INT          NOT NULL,
    guard_hit   TINYINT(1)   NOT NULL DEFAULT 0,
    started_at  DATETIME(3)  NOT NULL,
    finished_at DATETIME(3)  NOT NULL,
    PRIMARY KEY (id),
    KEY idx_reconcile_tenant_started (tenant_id, started_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
