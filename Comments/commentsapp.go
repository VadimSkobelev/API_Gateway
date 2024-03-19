package main

import (
	"APIGateway/Comments/api"
	"APIGateway/Comments/storage"
	"fmt"
	"log"
	"net/http"
	"os"
)

var port = os.Getenv("API_PORT")

func main() {

	// Создаём канал для агрегации ошибок.
	chErrs := make(chan error)

	// Обработка потока ошибок.
	go func() {
		for err := range chErrs {
			log.Println("ERROR: ", err)
		}
	}()

	// Реляционная БД PostgreSQL.
	db, err := storage.New()
	if err != nil {
		chErrs <- fmt.Errorf("ошибка подключения к БД:  %v", err)
	}

	api := api.New(db)

	// запуск веб-сервера с API
	httpStart := fmt.Sprintf("HTTP server is started on localhost:%s", port)
	fmt.Println(httpStart)
	errLS := (http.ListenAndServe(":"+port, api.Router()))
	if errLS != nil {
		httpStartErr := fmt.Sprintf("HTTP server has been stopped. Reason: %v", errLS)
		fmt.Println(httpStartErr)
	}
}
