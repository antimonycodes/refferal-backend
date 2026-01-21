-- Drop the existing check constraint
ALTER TABLE referrals DROP CONSTRAINT referrals_status_check;

-- Add updated check constraint including 'rejected'
ALTER TABLE referrals ADD CONSTRAINT referrals_status_check CHECK (status IN ('pending', 'paid', 'rejected'));
