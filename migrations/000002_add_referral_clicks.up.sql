-- Add referral link click tracking table

CREATE TABLE IF NOT EXISTS referral_clicks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    referral_code VARCHAR(50) NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for counting clicks by referral code
CREATE INDEX IF NOT EXISTS idx_referral_clicks_referral_code ON referral_clicks(referral_code);
CREATE INDEX IF NOT EXISTS idx_referral_clicks_user_id ON referral_clicks(user_id);
CREATE INDEX IF NOT EXISTS idx_referral_clicks_created_at ON referral_clicks(created_at);
