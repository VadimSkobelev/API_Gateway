--Схема БД для комментариев.

DROP TABLE IF EXISTS comments;

-- комментарии
CREATE TABLE comments (
    id SERIAL PRIMARY KEY,
    news_id INT,
    comment TEXT,
    parent_comment_id INT,
    pub_time INTEGER DEFAULT 0
);

INSERT INTO comments (id) VALUES (0);