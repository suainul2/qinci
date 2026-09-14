# Qinci - GitHub Webhook Auto-Pull & Post-Command Runner

Aplikasi server berbasis **Golang** untuk menangani webhook GitHub (push event), melakukan **automated `git pull`** ke folder lokal server, dan menjalankan **custom post-pull commands** (seperti `npm install`, `npm run build`, dsb.). Dilengkapi dengan antarmuka web modern berbasis **Tailwind CSS** dan sistem autentikasi multi-user dengan database **MySQL**.

---

## 🚀 Fitur Utama

- **GitHub Webhook Handler**: Memproses event `push` dari GitHub secara otomatis.
- **HMAC SHA-256 Signature Verification**: Verifikasi keamanan payload menggunakan `X-Hub-Signature-256` untuk mencegah unauthorized request.
- **Asynchronous Execution**: Respon HTTP `200 OK` dikirim seketika ke GitHub untuk menghindari batas timeout webhook (10 detik), sementara `git pull` dan perintah build berjalan di latar belakang (background worker).
- **Custom Post-Pull Commands**: Mendukung perintah bertahap per baris (misal: `npm install`, `npm run build`, `composer install`).
- **Cross-Platform**: Eksekusi perintah otomatis mendeteksi shell host (`cmd.exe /C` di Windows atau `/bin/sh -c` di Linux/macOS).
- **Multi-User Dashboard Web**: Setiap pengguna dapat mengelola repositori masing-masing secara terisolasi.
- **Manual Trigger**: Fitur tombol **"Tarik Sekarang"** di web UI untuk menjalankan pull & build secara instan tanpa menunggu push webhook.
- **Environment Mode**: Mendukung mode `local` dan `production` (pada mode `production`, pendaftaran akun baru `/register` otomatis diblokir).
- **Keamanan Kredensial**: Password/Token disensor (`********`) pada setiap log stdout/stderr eksekusi git.

---

## 📋 Prasyarat Sistem

1. **Go** (Golang) versi 1.22 atau yang lebih baru.
2. **Git CLI** terpasang di server dan terdaftar dalam `PATH`.
3. **MySQL Server** (versi 5.7+ atau 8.0+ / MariaDB).
4. Node.js / NPM (opsional, jika menjalankan perintah `npm run build`).

---

## 🛠️ Instalasi & Konfigurasi

### 1. Clone atau Buka Folder Proyek
Masuk ke direktori proyek:
```bash
cd qinci
```

### 2. Setup Database MySQL
Buka MySQL CLI atau tool GUI (phpMyAdmin / DBeaver / Navicat), lalu buat database baru dan import file `schema.sql`:
```sql
CREATE DATABASE github_webhook CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE github_webhook;
```
Jalankan isi dari file `schema.sql`:
```bash
mysql -u root -p github_webhook < schema.sql
```

### 3. Konfigurasi File `.env`
Salin file `.env.example` menjadi `.env`:

**Windows (PowerShell):**
```powershell
Copy-Item .env.example .env
```
**Linux / macOS:**
```bash
cp .env.example .env
```

Buka file `.env` dan sesuaikan nilainya:
```ini
# Mode Environment: "local" atau "production"
# - local      : Pendaftaran user baru (/register) dibuka.
# - production : Pendaftaran user baru diblokir demi keamanan.
APP_ENV=local

# Port aplikasi server
APP_PORT=8080

# Konfigurasi Database MySQL
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=password_mysql_anda
DB_NAME=github_webhook

# Default Global Secret untuk Webhook (fallback jika secret repo kosong)
GLOBAL_WEBHOOK_SECRET=rahasia_global_anda_123
```

---

## ▶️ Menjalankan Aplikasi

### Mode Pengembangan (Development)
```bash
go run main.go
```

### Kompilasi ke Binary (Production)
**Windows:**
```powershell
go build -o qinci.exe main.go
.\qinci.exe
```

**Linux:**
```bash
go build -o qinci main.go
chmod +x qinci
./qinci
```

Setelah server aktif, Anda akan melihat log:
```
==================================================
 GitHub Webhook Auto-Pull & Post-Command Runner  
==================================================
[CONFIG INFO] Berhasil memuat konfigurasi dari file .env
[DATABASE] Koneksi ke MySQL berhasil dan siap digunakan.
[SERVER] Dashboard Web UI: http://localhost:8080/login
[SERVER] Webhook Endpoint : POST http://0.0.0.0:8080/webhook
```

---

## 🖥️ Panduan Penggunaan Web UI

### 1. Registrasi Akun
1. Buka browser ke: `http://localhost:8080/login`.
2. Klik tautan **"Daftar sekarang"** (pastikan `APP_ENV=local`).
3. Masukkan Nama Lengkap, Username, dan Password (minimal 6 karakter).
4. Setelah berhasil, Anda akan dialihkan ke halaman login.

> **Tips Keamanan Production**: Setelah membuat akun admin/utama Anda, ubah `APP_ENV=production` pada file `.env` dan restart server untuk mencegah pihak lain mendaftar akun baru.

### 2. Menambahkan Konfigurasi Repositori
1. Masuk ke Dashboard, lalu klik tombol **"Tambah Repositori"**.
2. Isi formulir dengan data yang sesuai:
   - **Nama Repository GitHub**: Nama lengkap repositori di GitHub, misalnya `username/nama-repo`.
   - **Branch Target**: Branch yang ingin dipantau, misalnya `main` atau `production`.
   - **GitHub Username**: Username akun GitHub Anda.
   - **Personal Access Token (PAT) / Password**: Token GitHub Anda dengan permission `repo` (format: `ghp_xxxxxxxxxxxx`).
   - **Relative Path ke Folder Lokal Server**: Path folder tempat repositori lokal berada di server (contoh: `./repos/my-web-app` atau `D:/work/apps/my-web-app`).
     > **PENTING**: Folder target di server harus sudah di-clone sebelumnya dan memiliki folder `.git`.
   - **Webhook Secret**: Kunci rahasia unik (HMAC-SHA256) yang akan Anda samakan di setting webhook GitHub.
   - **Custom Post-Pull Commands**: Perintah yang ingin dijalankan setelah `git pull` berhasil, pisahkan per baris. Contoh:
     ```bash
     npm install
     npm run build
     ```
   - **Status Aktif**: Centang agar webhook untuk repositori ini aktif.
3. Klik **"Tambah Repositori"**.

### 3. Menguji Tarik Manual (Manual Trigger)
Pada card repositori di Dashboard, Anda dapat menekan tombol **"Tarik Sekarang"**. Server akan langsung mengeksekusi `git pull` dan rangkaian perintah build di background tanpa perlu melakukan push ke GitHub.

---

## 🔗 Konfigurasi Webhook di GitHub

1. Buka repositori Anda di GitHub.
2. Masuk ke tab **Settings** > **Webhooks** > klik **Add webhook**.
3. Isi parameter berikut:
   - **Payload URL**: `http://<IP_ATAU_DOMAIN_SERVER>:8080/webhook` *(contoh jika menggunakan domain/ngrok: `https://webhook.domainanda.com/webhook`)*
   - **Content type**: Pilih `application/json`.
   - **Secret**: Masukkan secret yang sama persis dengan yang Anda daftarkan di dashboard untuk repositori tersebut.
   - **SSL verification**: Enable (jika menggunakan HTTPS).
   - **Which events would you like to trigger this webhook?**: Pilih `Just the push event`.
   - Centang **Active**.
4. Klik **Add webhook**.

---

## 🧪 Menjalankan Pengujian Unit (Unit Tests)

Aplikasi dilengkapi dengan test suite untuk memvalidasi algoritma verifikasi HMAC, shell command runner, dan proteksi middleware:

```bash
go test -v ./...
```

---

## 📂 Struktur Direktori Proyek

```
qinci/
├── .env                               # File konfigurasi lokal (tidak di-commit ke Git)
├── .env.example                       # Template konfigurasi environment
├── go.mod                             # Definisi dependensi Golang
├── go.sum                             # Checksum dependensi
├── main.go                            # Entry point aplikasi & HTTP routing
├── README.md                          # Dokumentasi petunjuk penggunaan
├── schema.sql                         # Skema database MySQL (users & repositories)
├── templates/                         # Antarmuka Web (Go HTML Templates)
│   ├── login.html                     # Halaman login
│   ├── register.html                  # Halaman pendaftaran user
│   ├── index.html                     # Dashboard daftar repositori
│   └── form.html                      # Form tambah & edit repositori
└── internal/
    ├── config/                        # Handler parsing environment & .env file
    ├── db/                            # Koneksi MySQL & database stores (user & repo)
    ├── model/                         # Struct entity User & RepositoryConfig
    ├── runner/                        # Git pull engine & post-command executor
    ├── session/                       # Session manager berbasis cookie HttpOnly aman
    ├── web/                           # Handler web UI & middleware autentikasi
    └── webhook/                       # Handler webhook GitHub & verifikasi HMAC-SHA256
```

---

## 📄 Lisensi

Proyek ini bersifat open-source dan bebas dimodifikasi sesuai kebutuhan deployment Anda.
