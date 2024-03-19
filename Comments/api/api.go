package api

import (
	"APIGateway/Comments/storage"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type Comment struct {
	ID              int    `json:"ID"`              // уникальный идентификатор комментария
	NewsID          int    `json:"NewsID"`          // уникальный идентификатор новости
	Comment         string `json:"Comment"`         // текст комментария
	ParentCommentID int    `json:"ParentCommentID"` // уникальный идентификатор родительского комментария
	PubTime         int64  `json:"PubTime"`         // время создания комментария (получаем от fontend)
}

type API struct {
	db *storage.Storage
	r  *mux.Router
}

// Конструктор API.
func New(db *storage.Storage) *API {
	a := API{db: db, r: mux.NewRouter()}
	a.endpoints()
	return &a
}

// Router возвращает маршрутизатор для использования
// в качестве аргумента HTTP-сервера.
func (api *API) Router() *mux.Router {
	return api.r
}

// Регистрация методов API в маршрутизаторе запросов.
func (api *API) endpoints() {
	api.r.HandleFunc("/comments", api.comments).Methods("GET")           // получение всех комментариев по id новости
	api.r.HandleFunc("/commentsCheck", api.commentsCheck).Methods("GET") // проверка наличия комментария в БД для новости
	api.r.HandleFunc("/add-comment", api.addComment).Methods("POST")     // добавление комментария к новости
}

// получение всех комментариев по id новости
func (api *API) comments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	uniqueReqID := r.URL.Query().Get("request_id")

	news_idSTR := r.URL.Query().Get("news_id")
	news_id, err := strconv.Atoi(news_idSTR)
	if err != nil {
		log.Printf("request_id %s: id новости в url %s: %v", uniqueReqID, news_idSTR, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	comments, err := api.db.Comments(news_id, uniqueReqID)
	if err != nil {
		log.Printf("request_id %s: комментарии для новости %d не получены из БД: %v", uniqueReqID, news_id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if comments != nil {
		json.NewEncoder(w).Encode(comments)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// проверка наличия комментария в БД для новости
func (api *API) commentsCheck(w http.ResponseWriter, r *http.Request) {

	uniqueReqID := r.URL.Query().Get("request_id")

	p_comment_idSTR := r.URL.Query().Get("p_comment_id")
	p_comment_id, err := strconv.Atoi(p_comment_idSTR)
	if err != nil {
		log.Printf("request_id %s: id родительского комментария в url %s: %v", uniqueReqID, p_comment_idSTR, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	news_idSTR := r.URL.Query().Get("news_id")
	news_id, err := strconv.Atoi(news_idSTR)
	if err != nil {
		log.Printf("request_id %s: id новости в url %s: %v", uniqueReqID, news_idSTR, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	check, err := api.db.CommentsCheck(p_comment_id, news_id, uniqueReqID)

	if err != nil {
		log.Printf("request_id %s: проверка наличия комментария в БД; получена ошибка %v", uniqueReqID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if check {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// добавление комментария к новости
func (api *API) addComment(w http.ResponseWriter, r *http.Request) {

	uniqueReqID := r.URL.Query().Get("request_id")

	var newComment storage.Comment

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newComment)
	if err != nil {
		log.Printf("request_id %s: ошибка декодирования json: %v", uniqueReqID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = api.db.AddComment(newComment, uniqueReqID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
