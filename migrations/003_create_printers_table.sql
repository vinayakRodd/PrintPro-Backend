-- Migration: Create Printers Table for Agent Sync
-- Created: 2025-01-XX
-- This migration creates a printers table to store printer information synced from agents
-- NOTE: This drops the old printers table from migration 001 if it exists, as it has a different structure

-- Drop the old printers table if it exists (from migration 001 - incompatible structure)
-- The old table referenced partners(id) and had different columns
DROP TABLE IF EXISTS printers CASCADE;

-- Create new printers table with the correct structure
CREATE TABLE printers (
    id SERIAL PRIMARY KEY,
    partner_id INT NOT NULL,                    -- Foreign Key to partner_profiles.id
    printer_name VARCHAR(255) NOT NULL,         -- Name of the printer
    serial_number VARCHAR(255) UNIQUE NOT NULL, -- Unique serial number (used for upsert)
    status VARCHAR(50) DEFAULT 'online',       -- Printer status (online, offline, etc.)
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- Last time printer was seen
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (partner_id) REFERENCES partner_profiles(id) ON DELETE CASCADE
);

-- Create index on partner_id for fast dashboard loading
CREATE INDEX IF NOT EXISTS idx_printers_partner_id ON printers(partner_id);

-- Create index on serial_number for fast lookups
CREATE INDEX IF NOT EXISTS idx_printers_serial_number ON printers(serial_number);

-- Create index on last_seen for status monitoring
CREATE INDEX IF NOT EXISTS idx_printers_last_seen ON printers(last_seen);

