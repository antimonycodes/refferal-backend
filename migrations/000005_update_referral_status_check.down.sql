-- Drop the new check constraint
ALTER TABLE referrals DROP CONSTRAINT referrals_status_check;

-- Revert to original check constraint
ALTER TABLE referrals ADD CONSTRAINT referrals_status_check CHECK (status IN ('pending', 'paid'));
