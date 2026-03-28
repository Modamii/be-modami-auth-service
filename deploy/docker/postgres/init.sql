-- Create application user and database for Keycloak
CREATE USER modami WITH PASSWORD 'modami';
CREATE DATABASE "be-modami-auth-service" OWNER modami;
GRANT ALL PRIVILEGES ON DATABASE "be-modami-auth-service" TO modami;
