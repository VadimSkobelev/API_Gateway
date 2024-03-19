// Пакет для обработки RSS-потока.
package rss

import (
	"APIGateway/NewsAggregator/storage"
	"encoding/xml"
	"io"
	"net/http"
	"time"
)

type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Chanel  Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Link        string `xml:"link"`
}

// Получаем и обрабатываем данные из RSS канала.
func ReadRSS(url string) ([]storage.NewsFullDetailed, error) {

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var f Feed
	err = xml.Unmarshal(body, &f)
	if err != nil {
		return nil, err
	}

	var postList []storage.NewsFullDetailed
	for _, item := range f.Chanel.Items {
		var p storage.NewsFullDetailed
		p.Title = item.Title
		p.Content = item.Description
		t, err := time.Parse(time.RFC1123, item.PubDate)
		if err != nil {
			t, _ = time.Parse(time.RFC1123Z, item.PubDate)
		}
		p.PubTime = t.Unix()
		p.Link = item.Link
		postList = append(postList, p)
	}
	return postList, nil
}
