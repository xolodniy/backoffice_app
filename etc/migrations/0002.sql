CREATE TABLE vacations
(
  user_id    VARCHAR                     NOT NULL PRIMARY KEY,
  date_start DATE                        NOT NULL,
  date_end   DATE                        NOT NULL,
  message    TEXT                        NOT NULL,
  created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);