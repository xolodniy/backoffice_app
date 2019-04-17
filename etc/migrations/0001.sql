CREATE TABLE commits
(
  id         BIGSERIAL                   NOT NULL PRIMARY KEY,
  type       VARCHAR                     NOT NULL,
  hash       VARCHAR                     NOT NULL,
  repository VARCHAR                     NOT NULL,
  path       VARCHAR                     NOT NULL,
  message    VARCHAR                     NOT NULL,
  created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE afk_timers
(
  id         BIGSERIAL                   NOT NULL PRIMARY KEY,
  user_id    VARCHAR                     NOT NULL,
  duration   VARCHAR                     NOT NULL,
  updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);