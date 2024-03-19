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

type Pagination struct {
	Page       int `json:"Page"`       // текущий номер страницы
	NewsOnPage int `json:"NewsOnPage"` // количество заголовков новостей на странице
	TotalPages int `json:"TotalPages"` // общее число страниц
	TotalNews  int `json:"TotalNews"`  // общее число новостей в БД
}

// Коротко описывает новость для списка новостей.
type NewsShortDetailed struct {
	ID      int    `json:"ID"`      // уникальный идентификатор новости
	Title   string `json:"Title"`   // заголовок новости
	PubTime int64  `json:"PubTime"` // время новости
}

// Структура для ответа на запрос списка новостей. С пагинацией.
type PaginationNewsList struct {
	NewsList       []NewsShortDetailed `json:"NewsList"`
	PaginationInfo Pagination          `json:"PaginationInfo"`
	Error          int
}

// Детальная информация по новости.
type NewsFullDetailed struct {
	ID      int    // уникальный идентификатор новости
	Title   string // заголовок новости
	Content string // содержание новости
	PubTime int64  // время публикации новости
	Link    string // ссылка на источник
	Error   int    // данное поле служит для информирования клиента об ошибке
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

// NewsList возвращает n новостей из БД для указанной страницы.
func (s *Storage) NewsList(amount, page int, search, uniqueReqID string) (PaginationNewsList, error) {
	offset := amount * (page - 1)
	rows, err := s.db.Query(context.Background(), `
		SELECT 
			id,
			title,
			pub_time
		FROM news
		WHERE
		title ILIKE '%'||$3||'%'
		ORDER BY pub_time DESC
		LIMIT $1
		OFFSET $2;
	`, amount, offset, search,
	)
	if err != nil {
		log.Printf("request_id %s: ошибка запроса в БД (получения списка новостей): %v", uniqueReqID, err)
		return PaginationNewsList{}, err
	}
	var news []NewsShortDetailed
	// итерирование по результату выполнения запроса
	// и сканирование каждой строки в переменную
	for rows.Next() {
		var p NewsShortDetailed
		err = rows.Scan(
			&p.ID,
			&p.Title,
			&p.PubTime,
		)
		if err != nil {
			log.Printf("request_id %s: ошибка чтения полученных данных из БД (список новостей): %v", uniqueReqID, err)
			return PaginationNewsList{}, err
		}
		// добавление переменной в массив результатов
		news = append(news, p)
	}
	var pag Pagination
	rowsP, err := s.db.Query(context.Background(), `
	SELECT 
		count(id)
	FROM 
		news
	WHERE
		title ILIKE '%'||$1||'%';;
`, search,
	)
	if err != nil {
		log.Printf("request_id %s: ошибка запроса в БД (подсчёт общего числа новостей): %v", uniqueReqID, err)
		return PaginationNewsList{}, err
	}
	for rowsP.Next() {
		err = rowsP.Scan(
			&pag.TotalNews,
		)
		if err != nil {
			log.Printf("request_id %s: ошибка чтения полученных данных из БД (общее число новостей): %v", uniqueReqID, err)
			return PaginationNewsList{}, err
		}
	}
	pag.Page = page
	pag.NewsOnPage = amount
	if pag.TotalNews%amount != 0 {
		pag.TotalPages = (pag.TotalNews / amount) + 1
	} else {
		pag.TotalPages = pag.TotalNews / amount
	}
	var pagNewsList PaginationNewsList
	pagNewsList.NewsList = news
	pagNewsList.PaginationInfo = pag
	return pagNewsList, rows.Err()
}

// News возвращает полную новость из БД.
func (s *Storage) News(news_id int, uniqueReqID string) (*NewsFullDetailed, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT 
			id,
			title,
			content,
			pub_time,
			link
		FROM news
		WHERE id=$1;
	`, news_id,
	)
	if err != nil {
		log.Printf("request_id %s: ошибка запроса в БД (получения новости %d): %v", uniqueReqID, news_id, err)
		return nil, err
	}
	var p NewsFullDetailed
	if rows == nil {
		return &p, nil
	}
	// итерирование по результату выполнения запроса
	// и сканирование каждой строки в переменную
	for rows.Next() {
		err = rows.Scan(
			&p.ID,
			&p.Title,
			&p.Content,
			&p.PubTime,
			&p.Link,
		)
		if err != nil {
			log.Printf("request_id %s: ошибка чтения полученных данных из БД (новость %d): %v", uniqueReqID, news_id, err)
			return nil, err
		}
	}
	return &p, rows.Err()
}

// AddNews добовляет новость в базу.
func (s *Storage) AddNews(p []NewsFullDetailed) error {
	for _, post := range p {
		err := s.db.QueryRow(context.Background(), `
		INSERT INTO news (title, content, pub_time, link)
		VALUES ($1, $2, $3, $4) RETURNING id;
		`,
			post.Title,
			post.Content,
			post.PubTime,
			post.Link,
		).Scan(&post.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewsCheck проверяет наличие новости в БД.
func (s *Storage) NewsCheck(news_id int, uniqueReqID string) (bool, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT 
			id
		FROM news
		WHERE id=$1;
	`, news_id,
	)
	if err != nil {
		log.Printf("request_id %s: ошибка запроса в БД (проверка наличия новости %d): %v", uniqueReqID, news_id, err)
		return false, err
	}
	var p NewsFullDetailed
	for rows.Next() {
		err = rows.Scan(
			&p.ID,
		)
		if err != nil {
			log.Printf("request_id %s: ошибка чтения полученных данных из БД (проверка наличия новости %d): %v", uniqueReqID, news_id, err)
			return false, err
		}
	}
	if p.ID != 0 {
		return true, rows.Err()
	} else {
		return false, rows.Err()
	}
}
