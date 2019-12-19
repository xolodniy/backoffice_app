CREATE TABLE rb_auth
(
    tg_user_id BIGINT                      NOT NULL PRIMARY KEY,
    username   TEXT                        NOT NULL,
    first_name TEXT                        NOT NULL,
    last_name  TEXT                        NOT NULL,
    projects   TEXT[]                      NOT NULL DEFAULT '{}'::TEXT[],
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL
)
;