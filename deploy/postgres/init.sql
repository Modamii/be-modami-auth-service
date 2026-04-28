-- Create application user and database
CREATE USER modami WITH PASSWORD 'modami';
CREATE DATABASE "be-modami-auth-service" OWNER modami;
GRANT ALL PRIVILEGES ON DATABASE "be-modami-auth-service" TO modami;

\connect "be-modami-auth-service"

-- Grant schema-level permissions so migrations can run
GRANT ALL ON SCHEMA public TO modami;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO modami;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO modami;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO modami;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO modami;
