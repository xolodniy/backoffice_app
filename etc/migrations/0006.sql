CREATE TABLE on_duty_users
(
    id            SERIAL PRIMARY KEY,
    slack_user_id VARCHAR NOT NULL,
    team          VARCHAR NOT NULL
)