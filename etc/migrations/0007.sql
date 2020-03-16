CREATE TABLE protected_branches (
    id    SERIAL PRIMARY KEY,
    name    VARCHAR NOT NULL,
    comment VARCHAR NOT NULL,
    user_id VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP WITH TIME ZONE
);
