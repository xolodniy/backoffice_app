CREATE TABLE reminders
(
    id          SERIAL                      NOT NULL PRIMARY KEY,
    user_id     VARCHAR                     NOT NULL,
    message     VARCHAR                     NOT NULL,
    channel_id  VARCHAR                     NOT NULL,
    thread_ts   VARCHAR                     NOT NULL,
    reply_count INT                         NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);