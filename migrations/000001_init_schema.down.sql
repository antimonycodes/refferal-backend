-- Rollback initial schema

DROP INDEX IF EXISTS idx_payouts_status;
DROP INDEX IF EXISTS idx_payouts_user_id;
DROP INDEX IF EXISTS idx_referrals_status;
DROP INDEX IF EXISTS idx_referrals_referrer_id;
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_referral_code;
DROP INDEX IF EXISTS idx_users_email;

DROP TABLE IF EXISTS payouts;
DROP TABLE IF EXISTS referrals;
DROP TABLE IF EXISTS users;
