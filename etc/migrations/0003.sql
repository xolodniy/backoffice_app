CREATE TABLE rb_auth (
    tg_user_id BIGINT NOT NULL PRIMARY KEY,
    projects TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    updated_at  TIMESTAMP WITHOUT TIME ZONE NOT NULL
)
;