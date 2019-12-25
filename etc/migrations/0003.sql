CREATE TABLE forgotten_pull_requests
(
    pull_request_id INT                         NOT NULL,
    repo_slug       VARCHAR                     NOT NULL,
    created_at      TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pull_request_id, repo_slug)
);

CREATE TABLE forgotten_branches
(
    name          VARCHAR                     NOT NULL,
    repo_slug     VARCHAR                     NOT NULL,
    created_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (name, repo_slug)
);