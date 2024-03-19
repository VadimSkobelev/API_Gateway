package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/gorilla/mux"
	"github.com/rs/xid"
)

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
	Error          int                 `json:"Error"` // данное поле служит для информирования клиента об ошибке
}

// Детальная информация по новости.
type NewsFullDetailed struct {
	ID      int    `json:"ID"`      // уникальный идентификатор новости
	Title   string `json:"Title"`   // заголовок новости
	Content string `json:"Content"` // содержание новости
	PubTime int64  `json:"PubTime"` // время публикации новости
	Link    string `json:"Link"`    // ссылка на источник
	Error   int    `json:"Error"`   // данное поле служит для информирования клиента об ошибке
}

type Comment struct {
	ID              int    `json:"ID"`              // уникальный идентификатор комментария
	NewsID          int    `json:"NewsID"`          // уникальный идентификатор новости
	Comment         string `json:"Comment"`         // текст комментария
	ParentCommentID int    `json:"ParentCommentID"` // уникальный идентификатор родительского комментария
	PubTime         int64  `json:"PubTime"`         // время создания комментария (получаем от fontend)
	Error           int    `json:"Error"`           // данное поле служит для информирования клиента об ошибке
}

// Детальная информация по новости для структуры NewsComments. Без ошибки.
type News struct {
	ID      int    `json:"ID"`      // уникальный идентификатор новости
	Title   string `json:"Title"`   // заголовок новости
	Content string `json:"Content"` // содержание новости
	PubTime int64  `json:"PubTime"` // время публикации новости
	Link    string `json:"Link"`    // ссылка на источник
}

// Для структуры NewsComments. Без ошибки.
type Comments struct {
	ID              int    `json:"ID"`              // уникальный идентификатор комментария
	NewsID          int    `json:"NewsID"`          // уникальный идентификатор новости
	Comment         string `json:"Comment"`         // текст комментария
	ParentCommentID int    `json:"ParentCommentID"` // уникальный идентификатор родительского комментария
	PubTime         int64  `json:"PubTime"`         // время создания комментария (получаем от fontend)
}

type NewsComments struct {
	N     News       `json:"News"`
	C     []Comments `json:"Comments"`
	Error int        `json:"Error"` // данное поле служит для информирования клиента об ошибке
}

type ctxKey string

const uniqueID ctxKey = "unique_id"

var port = os.Getenv("API_PORT")
var chErrs chan error

func main() {

	// Создаём канал для агрегации ошибок.
	chErrs = make(chan error)

	// Обработка потока ошибок.
	go func() {
		for err := range chErrs {
			log.Println("ERROR:", err)
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/newsList", myMiddleware(newsList)).Methods("GET")       // получение списка новостей
	r.HandleFunc("/news", myMiddleware(fullNews)).Methods("GET")           // получение новости по id
	r.HandleFunc("/comment", myMiddleware(comment)).Methods("GET")         // получение всех комментариев по id новости
	r.HandleFunc("/add-comment", myMiddleware(addComment)).Methods("POST") // добавление комментария к новости
	r.HandleFunc("/news+comments", myMiddleware(getFull)).Methods("GET")   // получение новости со всеми комментариями
	http.Handle("/", r)
	httpStart := fmt.Sprintf("HTTP server is started on localhost:%s", port)
	fmt.Println(httpStart)
	errLS := (http.ListenAndServe(":"+port, r))
	if errLS != nil {
		httpStartErr := fmt.Sprintf("HTTP server has been stopped. Reason: %v", errLS)
		fmt.Println(httpStartErr)
	}
}

func myMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var uniqueReqID string // уникальный номер запроса

		request_id := r.URL.Query().Get("request_id")
		if request_id != "" {
			uniqueReqID = request_id
		} else {
			uniqueReqID = xid.New().String()
		}

		ctx := context.WithValue(r.Context(), uniqueID, uniqueReqID)
		requestTime := time.Now().Format("2006-01-02 15:04:05")
		ip := r.RemoteAddr
		url := r.URL
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next(ww, r.WithContext(ctx))

		log.Printf("Timestamp: %s, IP: %s, Unique ID: %s, URL запроса: %s, HTTP Response Code: %d", requestTime, ip, uniqueReqID, url, ww.Status())
	}
}

// получение списка новостей
func newsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uniqueReqID := r.Context().Value(uniqueID).(string)

	var returnError PaginationNewsList

	amountSTR := r.URL.Query().Get("amount")
	if amountSTR == "" {
		amountSTR = "10"
	}
	amount, err := strconv.Atoi(amountSTR)
	if err != nil || amount <= 0 {
		chErrs <- fmt.Errorf("request_id %s: количество запрашиваемых новостей в url %s: %v", uniqueReqID, amountSTR, err)
		returnError.Error = http.StatusBadRequest
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	pageSTR := r.URL.Query().Get("page")
	if pageSTR == "" {
		pageSTR = "1"
	}
	page, err := strconv.Atoi(pageSTR)
	if err != nil || page <= 0 {
		chErrs <- fmt.Errorf("request_id %s: запрашиваемая страница в url %s: %v", uniqueReqID, pageSTR, err)
		returnError.Error = http.StatusBadRequest
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	search := r.URL.Query().Get("search")

	url := fmt.Sprintf("http://news:8081/newsList?amount=%d&page=%d&search=%s&request_id=%s", amount, page, search, uniqueReqID)
	resp, err := http.Get(url)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в news: %v", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		chErrs <- fmt.Errorf("request_id %s: ответ от news (status code): %d", uniqueReqID, resp.StatusCode)
		returnError.Error = resp.StatusCode
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка чтения тела ответа от news : %s", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	w.Write(body)

}

// получение новости по id
func fullNews(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uniqueReqID := r.Context().Value(uniqueID).(string)

	var returnError NewsFullDetailed

	news_idSTR := r.URL.Query().Get("news_id")
	news_id, err := strconv.Atoi(news_idSTR)
	if err != nil || news_id <= 0 {
		chErrs <- fmt.Errorf("request_id %s: id новости в url %s: %v", uniqueReqID, news_idSTR, err)
		returnError.Error = http.StatusBadRequest
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	url := fmt.Sprintf("http://news:8081/news?news_id=%d&request_id=%s", news_id, uniqueReqID)
	resp, err := http.Get(url)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в news: %v", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		chErrs <- fmt.Errorf("request_id %s: ответ от news (status code): %d", uniqueReqID, resp.StatusCode)
		returnError.Error = resp.StatusCode
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка чтения тела ответа от news : %s", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	w.Write(body)
}

// получение всех комментариев по id новости
func comment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uniqueReqID := r.Context().Value(uniqueID).(string)

	var returnError Comment

	news_idSTR := r.URL.Query().Get("news_id")
	news_id, err := strconv.Atoi(news_idSTR)
	if err != nil || news_id <= 0 {
		chErrs <- fmt.Errorf("request_id %s: id новости в url %s: %v", uniqueReqID, news_idSTR, err)
		returnError.Error = http.StatusBadRequest
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	url := fmt.Sprintf("http://comments:8082/comments?news_id=%d&request_id=%s", news_id, uniqueReqID)
	resp, err := http.Get(url)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в comments: %v", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		chErrs <- fmt.Errorf("request_id %s: ответ от comments (status code): %d", uniqueReqID, resp.StatusCode)
		returnError.Error = resp.StatusCode
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка чтения тела ответа от comments : %s", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}
	w.Write(body)
}

// добавление комментария к новости
func addComment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uniqueReqID := r.Context().Value(uniqueID).(string)

	// Разбираем полученный json.
	var C Comment
	json.NewDecoder(r.Body).Decode(&C)
	defer r.Body.Close()

	newData, err := json.Marshal(C)
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка выполнения маршалинга", uniqueReqID)
	}

	// Асинхронный запуск:
	// - проверки наличия родительского комментария в БД
	// - проверки наличия новости в БД
	// - проверки комментария на запрещённые слова.
	var wg sync.WaitGroup
	checkChan := make(chan int, 3)

	// Проверяем наличие родительского комментария.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if C.ParentCommentID != 0 {
			urlCommentCheck := fmt.Sprintf("http://comments:8082/commentsCheck?p_comment_id=%d&news_id=%d&request_id=%s", C.ParentCommentID, C.NewsID, uniqueReqID)
			respCommentCheck, err := http.Get(urlCommentCheck)
			if err != nil {
				chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в comments: %v", uniqueReqID, err)
				checkChan <- http.StatusInternalServerError
				return
			}

			if respCommentCheck.StatusCode != http.StatusOK {
				chErrs <- fmt.Errorf("request_id %s: родительский комментарий (parent comment ID %d) отсутствует в БД или не ссответствует новости", uniqueReqID, C.ParentCommentID)
				checkChan <- respCommentCheck.StatusCode
			}
		}
	}()

	// Проверяем наличие новости в БД.
	wg.Add(1)
	go func() {
		defer wg.Done()
		urlNewsCheck := fmt.Sprintf("http://news:8081/newsCheck?news_id=%d&request_id=%s", C.NewsID, uniqueReqID)
		respNewsCheck, err := http.Get(urlNewsCheck)
		if err != nil {
			chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в news: %v", uniqueReqID, err)
			checkChan <- http.StatusInternalServerError
			return
		}

		if respNewsCheck.StatusCode != http.StatusOK {
			chErrs <- fmt.Errorf("request_id %s: новость (news ID %d) отсутствует в БД", uniqueReqID, C.NewsID)
			checkChan <- respNewsCheck.StatusCode
		}
	}()

	// Проверяем комментарий на наличие запрещённых слов.
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("http://verification:8083/verification?request_id=%s", uniqueReqID)
		check, err := http.Post(url, "application/json", bytes.NewBuffer(newData))
		if err != nil {
			chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в verification %v", uniqueReqID, err)
			checkChan <- http.StatusInternalServerError
			return
		}
		defer check.Body.Close()

		if check.StatusCode == http.StatusInternalServerError {
			checkChan <- http.StatusInternalServerError
		} else if check.StatusCode == http.StatusBadRequest {
			chErrs <- fmt.Errorf("request_id %s: комментарий не прошёл проверку (status code): %d", uniqueReqID, check.StatusCode)
			checkChan <- check.StatusCode
		}
	}()

	wg.Wait()
	close(checkChan)

	var returnError Comment
	for data := range checkChan {
		if data == 404 {
			returnError.Error = http.StatusNotFound
			w.WriteHeader(returnError.Error)
			json.NewEncoder(w).Encode(returnError)
			return
		}
		if data == 400 {
			returnError.Error = http.StatusBadRequest
			w.WriteHeader(returnError.Error)
			json.NewEncoder(w).Encode(returnError)
			return
		}
		if data == 500 {
			returnError.Error = http.StatusInternalServerError
			w.WriteHeader(returnError.Error)
			json.NewEncoder(w).Encode(returnError)
			return
		}
	}

	// Добавляем комментарий если проверки пройдены.
	url := fmt.Sprintf("http://comments:8082/add-comment?request_id=%s", uniqueReqID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(newData))
	if err != nil {
		chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в comments %v", uniqueReqID, err)
		returnError.Error = http.StatusInternalServerError
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		chErrs <- fmt.Errorf("request_id %s: ответ от comments (status code): %d", uniqueReqID, resp.StatusCode)
		returnError.Error = resp.StatusCode
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	} else {
		returnError.Error = 0
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(returnError)
	}
}

// получение новости со всеми комментариями
func getFull(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uniqueReqID := r.Context().Value(uniqueID).(string)

	var returnError NewsComments
	var news News
	var c []Comments
	var out NewsComments

	news_idSTR := r.URL.Query().Get("news_id")
	news_id, err := strconv.Atoi(news_idSTR)
	if err != nil || news_id <= 0 {
		chErrs <- fmt.Errorf("request_id %s: id новости в url %s: %v", uniqueReqID, news_idSTR, err)
		returnError.Error = http.StatusBadRequest
		w.WriteHeader(returnError.Error)
		json.NewEncoder(w).Encode(returnError)
		return
	}

	// Асинхронный запуск:
	// - получение новости по id
	// - получение всех комментариев к новости.
	var wg sync.WaitGroup
	var er int // StatusCode ошибки
	outChan := make(chan NewsComments, 2)

	// - получение новости по id
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("http://news:8081/news?news_id=%d&request_id=%s", news_id, uniqueReqID)
		resp, err := http.Get(url)
		if err != nil {
			chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в news: %v", uniqueReqID, err)
			er = http.StatusInternalServerError
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			chErrs <- fmt.Errorf("request_id %s: ответ от news (status code): %d", uniqueReqID, resp.StatusCode)
			er = resp.StatusCode
		}
		json.NewDecoder(resp.Body).Decode(&news)
		outChan <- NewsComments{N: news, Error: er}
	}()

	// получение всех комментариев к новости
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("http://comments:8082/comments?news_id=%d&request_id=%s", news_id, uniqueReqID)
		resp, err := http.Get(url)
		if err != nil {
			chErrs <- fmt.Errorf("request_id %s: ошибка отправки запроса в comments: %v", uniqueReqID, err)
			er = http.StatusInternalServerError
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			chErrs <- fmt.Errorf("request_id %s: ответ от comments (status code): %d", uniqueReqID, resp.StatusCode)
			// Если комментарии для новости не найдены (StatusCode=404), то возвращаем ошибку=0, чтобы не блокировать вывод news+comments.
			// Комментарии будут пустыми.
			er = 0
		} else {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				chErrs <- fmt.Errorf("request_id %s: ошибка чтения тела ответа от comments : %s", uniqueReqID, err)
				er = http.StatusInternalServerError
			}
			err = json.Unmarshal(body, &c)
			if err != nil {
				chErrs <- fmt.Errorf("request_id %s: ошибка выполнения демаршалинга", uniqueReqID)
				er = http.StatusInternalServerError
			}
			outChan <- NewsComments{C: c, Error: er}
		}
	}()

	wg.Wait()
	close(outChan)

	for data := range outChan {
		if data.Error != 0 {
			out.Error = data.Error
			break
		}
		if data.N.ID != 0 {
			out.N = data.N
		}
		if data.C != nil {
			out.C = data.C
		}
	}
	if out.Error != 0 {
		out.N = News{}
		out.C = []Comments{}
		w.WriteHeader(out.Error)
		json.NewEncoder(w).Encode(out)
	} else {
		json.NewEncoder(w).Encode(out)
	}
}
