CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    user_login TEXT NOT NULL,
    user_password TEXT NOT NULL,
    registration_date TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id SERIAL PRIMARY KEY,
    creator INTEGER NOT NULL,
    size INT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_creator FOREIGN KEY (creator) REFERENCES users(id)
);
