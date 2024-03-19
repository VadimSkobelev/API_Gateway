package api

import (
	"APIGateway/NewsAggregator/storage"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

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
	api.r.HandleFunc("/newsList", api.newsList).Methods("GET")   // получение списка новостей
	api.r.HandleFunc("/news", api.news).Methods("GET")           // получение новости по id
	api.r.HandleFunc("/newsCheck", api.newsCheck).Methods("GET") // проверка наличия новости в БД
}

// получение списка новостей
func (api *API) newsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	uniqueReqID := r.URL.Query().Get("request_id")

	amountSTR := r.URL.Query().Get("amount")
	amount, err := strconv.Atoi(amountSTR)
	if err != nil {
		log.Printf("request_id %s: количество запрашиваемых новостей в url %s: %v", uniqueReqID, amountSTR, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pageSTR := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageSTR)
	if err != nil {
		log.Printf("request_id %s: запрашиваемая страница в url %s: %v", uniqueReqID, pageSTR, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	search := r.URL.Query().Get("search")

	news, err := api.db.NewsList(amount, page, search, uniqueReqID)
	if err != nil {
		log.Printf("request_id %s: список новостей не получен из БД %v", uniqueReqID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(news)
}

// получение новости по id
func (api *API) news(w http.ResponseWriter, r *http.Request) {
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

	news, err := api.db.News(news_id, uniqueReqID)
	if err != nil {
		log.Printf("request_id %s: новость %d не получена из БД: %v", uniqueReqID, news_id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if news.ID != 0 {
		json.NewEncoder(w).Encode(news)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// проверка наличия новости в БД
func (api *API) newsCheck(w http.ResponseWriter, r *http.Request) {

	uniqueReqID := r.URL.Query().Get("request_id")

	news_idSTR := r.URL.Query().Get("news_id")
	news_id, err := strconv.Atoi(news_idSTR)
	if err != nil {
		log.Printf("request_id %s: id новости в url %s: %v", uniqueReqID, news_idSTR, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	check, err := api.db.NewsCheck(news_id, uniqueReqID)

	if err != nil {
		log.Printf("request_id %s: проверка наличия новости в БД; получена ошибка %v", uniqueReqID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if check {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}
