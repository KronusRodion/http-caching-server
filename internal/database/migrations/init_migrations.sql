CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    user_login TEXT UNIQUE NOT NULL,
    user_password TEXT NOT NULL,
    registration_date TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id SERIAL PRIMARY KEY,
    file_name TEXT NOT NULL,
    size INT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    json_data JSONB,
    creator INT NOT NULL,
    mime_type TEXT,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    file_path TEXT NOT NULL,
    CONSTRAINT fk_creator FOREIGN KEY (creator) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS grants (
    file_id INT NOT NULL,
    user_id INT NOT NULL,
    granted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (file_id, user_id),
    CONSTRAINT fk_file FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);


CREATE TABLE IF NOT EXISTS tokens (
    id SERIAL PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    user_id INT NOT NULL,
    expiry TIMESTAMP NOT NULL,
    CONSTRAINT fk_creator FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);