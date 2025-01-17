CREATE TABLE IF NOT EXISTS creatives (
    id bigserial PRIMARY KEY, 
    user_id bigint NOT NULL REFERENCES users on DELETE CASCADE,
    creative_url text NOT NULL,
    scheduled_at DATE NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);