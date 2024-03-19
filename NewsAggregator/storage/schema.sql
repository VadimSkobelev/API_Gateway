--Схема БД для агрегатора новостей.

DROP TABLE IF EXISTS news;

-- новости
CREATE TABLE news (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    pub_time INTEGER DEFAULT 0,
    link TEXT NOT NULL UNIQUE -- UNIQUE для link позволяет избежать дублирования новостей в БД.
);