CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    name text NOT NULL,
    phone_number text UNIQUE NOT NULL,
    version integer NOT NULL DEFAULT 1
);