package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
)

type Comment struct {
	ID              int    `json:"ID"`              // уникальный идентификатор комментария
	NewsID          int    `json:"NewsID"`          // уникальный идентификатор новости
	Comment         string `json:"Comment"`         // текст комментария
	ParentCommentID int    `json:"ParentCommentID"` // уникальный идентификатор родительского комментария
	PubTime         int64  `json:"PubTime"`         // время создания комментария (получаем от fontend)
}

var port = os.Getenv("API_PORT")
var newComment Comment

// Список запрещённых слов.
var badWord = [3]string{"qwerty", "йцукен", "zxvbnm"}

func main() {

	r := mux.NewRouter()
	r.HandleFunc("/verification", verification).Methods("POST") // проверка комментария на запрещённын слова.
	http.Handle("/", r)
	httpStart := fmt.Sprintf("HTTP server is started on localhost:%s", port)
	fmt.Println(httpStart)
	errLS := (http.ListenAndServe(":"+port, r))
	if errLS != nil {
		httpStartErr := fmt.Sprintf("HTTP server has been stopped. Reason: %v", errLS)
		fmt.Println(httpStartErr)
	}
}

// проверка комментария на запрещённын слова.
func verification(w http.ResponseWriter, r *http.Request) {

	uniqueReqID := r.URL.Query().Get("request_id")

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newComment)
	if err != nil {
		log.Printf("request_id %s: ошибка декодирования json: %v", uniqueReqID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// приводим комментарий к нижнему регистру для сокращения числа образцов ругательств.
	commentLower := strings.ToLower(newComment.Comment)

	for _, bad := range badWord {
		match, err := regexp.MatchString(bad, commentLower)
		if err != nil {
			log.Printf("request_id %s: ошибка в поиске совпадений: %v", uniqueReqID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if match {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
