CREATE TABLE forgotten_pull_requests
(
    id              SERIAL                      NOT NULL PRIMARY KEY,
    pull_request_id SERIAL                      NOT NULL,
    title           VARCHAR                     NOT NULL,
    author          VARCHAR                     NOT NULL,
    repo_slug       VARCHAR                     NOT NULL,
    href            VARCHAR                     NOT NULL,
    last_activity   TIMESTAMP WITHOUT TIME ZONE,
    created_at      TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE forgotten_branches
(
    id         SERIAL                      NOT NULL PRIMARY KEY,
    name       VARCHAR                     NOT NULL,
    author     VARCHAR                     NOT NULL,
    repo_slug  VARCHAR                     NOT NULL,
    href       VARCHAR                     NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);