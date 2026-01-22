UPDATE users SET bank_name = '' WHERE bank_name IS NULL;
UPDATE users SET account_number = '' WHERE account_number IS NULL;
UPDATE users SET account_name = '' WHERE account_name IS NULL;
