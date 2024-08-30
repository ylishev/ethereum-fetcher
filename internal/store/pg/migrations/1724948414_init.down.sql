-- Drop the indexes created
DROP INDEX IF EXISTS idx_transactions_user_id;
DROP INDEX IF EXISTS idx_users_username_password;

-- Drop the transactions table
DROP TABLE IF EXISTS transactions;

-- Drop the users table
DROP TABLE IF EXISTS users;