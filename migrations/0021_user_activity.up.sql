CREATE TABLE user_activity (
    id            SERIAL PRIMARY KEY,
    user_id       INT NOT NULL REFERENCES Users(user_id) ON DELETE CASCADE,
    activity_type VARCHAR(50) NOT NULL,
    ip_address    VARCHAR(45),
    user_agent    TEXT,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_activity_user_id    ON user_activity(user_id);
CREATE INDEX idx_user_activity_created_at ON user_activity(created_at DESC);
CREATE INDEX idx_user_activity_type       ON user_activity(activity_type);
