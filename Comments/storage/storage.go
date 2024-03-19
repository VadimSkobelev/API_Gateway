// Пакет для работы с БД PostgreSQL.
package storage

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Хранилище данных.
type Storage struct {
	db *pgxpool.Pool
}

type Comment struct {
	ID              int    `json:"ID"`              // уникальный идентификатор комментария
	NewsID          int    `json:"NewsID"`          // уникальный идентификатор новости
	Comment         string `json:"Comment"`         // текст комментария
	ParentCommentID int    `json:"ParentCommentID"` // уникальный идентификатор родительского комментария
	PubTime         int64  `json:"PubTime"`         // время создания комментария (получаем от fontend)
}

var Host = os.Getenv("DB_HOST")
var Port = os.Getenv("DB_PORT")
var User = os.Getenv("DB_USER")
var Password = os.Getenv("DB_PASSWORD")
var Database = os.Getenv("DB_NAME")

// Подключение к БД.
func New() (*Storage, error) {
	constr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", User, Password, Host, Port, Database)
	db, err := pgxpool.Connect(context.Background(), constr)
	if err != nil {
		return nil, err
	}
	s := Storage{
		db: db,
	}
	return &s, nil
}

// Comments возвращает все комментарии по ID новости.
func (s *Storage) Comments(news_id int, uniqueReqID string) ([]Comment, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT 
			id,
			news_id,
			comment,
			parent_comment_id,
			pub_time
		FROM comments
		WHERE news_id=$1
		ORDER BY pub_time DESC;
	`, news_id,
	)
	if err != nil {
		log.Printf("request_id %s: ошибка запроса в БД (получение комментариев): %v", uniqueReqID, err)
		return nil, err
	}

	var comments []Comment
	if rows == nil {
		return nil, nil
	}

	// итерирование по результату выполнения запроса
	// и сканирование каждой строки в переменную
	for rows.Next() {
		var c Comment
		err = rows.Scan(
			&c.ID,
			&c.NewsID,
			&c.Comment,
			&c.ParentCommentID,
			&c.PubTime,
		)
		if err != nil {
			log.Printf("request_id %s: ошибка чтения полученных данных из БД (комментарии): %v", uniqueReqID, err)
			return nil, err
		}
		// добавление переменной в массив результатов
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// CommentsCheck проверяет наличие ID родительского комментария в БД.
func (s *Storage) CommentsCheck(p_comment_id, news_id int, uniqueReqID string) (bool, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT 
			id,
			news_id
		FROM comments
		WHERE id=$1;
	`, p_comment_id,
	)
	if err != nil {
		log.Printf("request_id %s: ошибка запроса в БД (проверка наличия комментария): %v", uniqueReqID, err)
		return false, err
	}
	var c Comment
	for rows.Next() {
		err = rows.Scan(
			&c.ID,
			&c.NewsID,
		)
		if err != nil {
			log.Printf("request_id %s: ошибка чтения полученных данных из БД (проверка наличия комментария): %v", uniqueReqID, err)
			return false, err
		}
	}

	if c.ID != 0 && news_id == c.NewsID {
		return true, rows.Err()
	} else {
		return false, rows.Err()
	}
}

// AddComment добовляет комментарий в базу.
func (s *Storage) AddComment(c Comment, uniqueReqID string) error {
	err := s.db.QueryRow(context.Background(), `
		INSERT INTO comments (news_id, comment, parent_comment_id, pub_time)
		VALUES ($1, $2, $3, $4) RETURNING id;
		`,
		c.NewsID,
		c.Comment,
		c.ParentCommentID,
		c.PubTime,
	).Scan(&c.ID)
	if err != nil {
		log.Printf("request_id %s: ошибка чтения полученных данных из БД (добавление комментария): %v", uniqueReqID, err)
		return err
	}
	return nil
}
