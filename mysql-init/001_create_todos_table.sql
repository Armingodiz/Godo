-- Migration: Create todos table
-- Version: 001
-- Description: Initial table creation for todo items

CREATE TABLE IF NOT EXISTS todos (
    id VARCHAR(36) PRIMARY KEY,
    description TEXT NOT NULL,
    due_date TIMESTAMP NOT NULL,
    file_id VARCHAR(36) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_due_date (due_date),
    INDEX idx_created_at (created_at),
    INDEX idx_file_id (file_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci; 