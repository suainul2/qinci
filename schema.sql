-- Skema Database MySQL untuk GitHub Webhook Auto-Pull & Post-Command Runner

-- 1. Tabel Users untuk Autentikasi Pengguna Web
CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
    `username` VARCHAR(100) NOT NULL UNIQUE,
    `password_hash` VARCHAR(255) NOT NULL,
    `full_name` VARCHAR(150) NULL,
    `telegram_chat_id` VARCHAR(64) NULL COMMENT 'Chat ID Telegram untuk notifikasi bot',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Migrasi aman untuk database yang sudah ada:
-- ALTER TABLE `users` ADD COLUMN `telegram_chat_id` VARCHAR(64) NULL AFTER `full_name`;

-- 2. Tabel Repositories dengan Relasi ke Tabel Users (user_id)
CREATE TABLE IF NOT EXISTS `repositories` (
    `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id` BIGINT NOT NULL COMMENT 'ID pemilik repository dari tabel users',
    `repo_name` VARCHAR(255) NOT NULL COMMENT 'Nama repository GitHub, contoh: username/repo-name atau repo-name',
    `branch` VARCHAR(100) NOT NULL DEFAULT 'main' COMMENT 'Nama branch target, contoh: main, master, production',
    `username` VARCHAR(255) NOT NULL COMMENT 'Username GitHub untuk autentikasi git pull',
    `password` VARCHAR(500) NOT NULL COMMENT 'Personal Access Token (PAT) atau password GitHub',
    `relative_path` VARCHAR(500) NOT NULL COMMENT 'Path folder relatif lokal (contoh: ./repos/frontend atau ../my-project)',
    `webhook_secret` VARCHAR(255) NULL COMMENT 'Secret key webhook GitHub untuk validasi HMAC-SHA256',
    `post_commands` TEXT NULL COMMENT 'Daftar custom command setelah git pull (dipisah baris baru, contoh: npm install\nnpm run build)',
    `is_active` TINYINT(1) NOT NULL DEFAULT 1 COMMENT 'Status aktif repository (1 = aktif, 0 = nonaktif)',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_user_repo_branch` (`user_id`, `repo_name`, `branch`),
    INDEX `idx_repo_branch` (`repo_name`, `branch`),
    CONSTRAINT `fk_repositories_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Tabel Repository Logs untuk Merekam Hasil Eksekusi Pull & Build (Hitungan Hari)
CREATE TABLE IF NOT EXISTS `repository_logs` (
    `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
    `repository_id` BIGINT NOT NULL COMMENT 'ID repository dari tabel repositories',
    `trigger_type` VARCHAR(50) NOT NULL DEFAULT 'webhook' COMMENT 'Sumber pemicu: webhook atau manual',
    `status` ENUM('success', 'failed') NOT NULL DEFAULT 'success' COMMENT 'Status eksekusi: success atau failed',
    `output` MEDIUMTEXT NULL COMMENT 'Log lengkap hasil git pull dan custom post-commands',
    `error_message` TEXT NULL COMMENT 'Pesan ringkas error jika gagal',
    `duration_seconds` DECIMAL(10, 2) NOT NULL DEFAULT 0.00 COMMENT 'Durasi eksekusi dalam detik',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_repo_created` (`repository_id`, `created_at`),
    CONSTRAINT `fk_logs_repository` FOREIGN KEY (`repository_id`) REFERENCES `repositories` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Contoh user default (password default: 'admin123', di-hash dengan bcrypt):
-- INSERT INTO `users` (`username`, `password_hash`, `full_name`) 
-- VALUES ('admin', '$2a$10$7R1Yk8k6Z2W8QcE3k8k6Ze1k8k6Z2W8QcE3k8k6Ze1k8k6Z2W8QcE', 'Administrator')
-- ON DUPLICATE KEY UPDATE `username`=`username`;
