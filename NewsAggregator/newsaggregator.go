package main

import (
	"APIGateway/NewsAggregator/api"
	"APIGateway/NewsAggregator/rss"
	"APIGateway/NewsAggregator/storage"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgconn"
)

// Структура конфигурационного файла.
type config struct {
	UrlList []string `json:"rss"`            // лист URL RSS каналаов
	Period  int      `json:"request_period"` // период опроса
}

var port = os.Getenv("API_PORT")

func main() {

	// Создаём канал для публикаций.
	chPosts := make(chan []storage.NewsFullDetailed)

	// Создаём канал для агрегации ошибок.
	chErrs := make(chan error)

	// Обработка потока ошибок.
	go func() {
		for err := range chErrs {
			log.Println("ERROR: ", err)
		}
	}()

	// Читаем файл конфигурации со списком RSS URLs и периодом опроса.
	fileIn, err := os.ReadFile("./config.json")
	if err != nil {
		chErrs <- fmt.Errorf("ошибка чтения файла конфигурации (config.json):  %v", err)
	}

	// Формируем структуру конфигурации из считанного файла конфигурации.
	var config config
	err = json.Unmarshal(fileIn, &config)
	if err != nil {
		chErrs <- fmt.Errorf("ошибка демаршалинга файла конфигурации (config.json):  %v", err)
	}

	// Реляционная БД PostgreSQL.
	db, err := storage.New()
	if err != nil {
		chErrs <- fmt.Errorf("ошибка подключения к БД:  %v", err)
	}

	api := api.New(db)

	// Проходим по списку RSS ссылок.
	// Для каждого RSS-канала запускается своя горутина.
	for _, url := range config.UrlList {
		go parseURL(url, chPosts, chErrs, config.Period)
	}

	// запись потока новостей в БД
	go func() {
		for posts := range chPosts {
			err := db.AddNews(posts)
			// Исключаем логирование ожидаемой ошибки записи дубликата новости в БД
			// "ERROR: duplicate key value violates unique constraint \"news_link_key\" (SQLSTATE 23505)"
			// в соответствии с правилом schema.sql
			// link TEXT NOT NULL UNIQUE -- UNIQUE для link позволяет избежать дублирования новостей в БД.
			if err != nil && err.(*pgconn.PgError).Code != "23505" {
				chErrs <- fmt.Errorf("ошибка при добавлении новости в БД:  %v", err)
			}
		}
	}()

	// запуск веб-сервера с API
	httpStart := fmt.Sprintf("HTTP server is started on localhost:%s", port)
	fmt.Println(httpStart)
	errLS := (http.ListenAndServe(":"+port, api.Router()))
	if errLS != nil {
		httpStartErr := fmt.Sprintf("HTTP server has been stopped. Reason: %v", errLS)
		fmt.Println(httpStartErr)
	}
}

// Асинхронное чтение потока RSS. Раскодированные новости и ошибки пишутся в каналы.
func parseURL(url string, posts chan<- []storage.NewsFullDetailed, errs chan<- error, period int) {
	for {
		news, err := rss.ReadRSS(url)
		if err != nil {
			errs <- fmt.Errorf("новости по ссылке %s не получены:  %v", url, err)
			continue
		}
		posts <- news
		time.Sleep(time.Minute * time.Duration(period))
	}
}
