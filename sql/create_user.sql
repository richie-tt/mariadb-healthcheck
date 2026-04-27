-- Replace 'healthcheck' (the password literal below) with a strong, unique
-- secret. The mariadb-healthcheck sidecar requires DB_PASSWORD to be set
-- explicitly — there is no fallback default.
CREATE USER 'healthcheck'@'127.0.0.1' IDENTIFIED BY 'healthcheck';
GRANT ALL PRIVILEGES ON `healthcheck`.* TO 'healthcheck'@'127.0.0.1';
