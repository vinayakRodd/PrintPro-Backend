-- Migration: Create Identity-Profile Pattern Tables
-- Created: 2025-01-XX
-- This migration implements the Identity-Profile pattern for better separation of concerns

-- 1. Create the Central Identity Table
CREATE TABLE IF NOT EXISTS accounts (
    id SERIAL PRIMARY KEY,               -- Numerical ID (Fast & Fixed)
    email VARCHAR(255) UNIQUE NOT NULL,  -- Login Identity
    password_hash TEXT NOT NULL,         -- Security
    user_type VARCHAR(20) NOT NULL,      -- 'partner' or 'customer'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Create the Partner Profile Table (Links to Accounts)
CREATE TABLE IF NOT EXISTS partner_profiles (
    id SERIAL PRIMARY KEY,
    account_id INT NOT NULL,             -- Foreign Key
    shop_name VARCHAR(100) NOT NULL,
    printer_id VARCHAR(50),              -- Unique ID for the Python Agent
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

-- 3. Create the Customer Profile Table (Links to Accounts)
CREATE TABLE IF NOT EXISTS customer_profiles (
    id SERIAL PRIMARY KEY,
    account_id INT NOT NULL,             -- Foreign Key
    phone_number VARCHAR(20),
    wallet_balance DECIMAL(10, 2) DEFAULT 0.00,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
CREATE INDEX IF NOT EXISTS idx_accounts_user_type ON accounts(user_type);
CREATE INDEX IF NOT EXISTS idx_partner_profiles_account_id ON partner_profiles(account_id);
CREATE INDEX IF NOT EXISTS idx_customer_profiles_account_id ON customer_profiles(account_id);

