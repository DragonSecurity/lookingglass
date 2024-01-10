CREATE TABLE users (
                       id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                       created TIMESTAMP NOT NULL,
                       email TEXT NOT NULL UNIQUE,
                       hashed_password TEXT NOT NULL
);
