ALTER TABLE forgotten_branches ADD COLUMN protected BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE forgotten_branches ADD COLUMN comment   TEXT;