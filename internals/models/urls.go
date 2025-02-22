package models

import (
	"encoding/json"
	"fmt"
	"go-shortURL/db"
	"time"
)

// type ShortenerData struct {
//     OriginalURL, ShortenedURL string
//     Clicks                	int
// }

// type ShortenerDataModel struct {
//     DB *db.LevelDBClient
// }

type URLModel struct {
    dbClient *db.LevelDBClient
}

type URLDate struct {
    OriginalURL string `json:"Original_url"`
    ShortenedURL  string `json:"shortened_url"` 
    CreatedAt   time.Time `json:"created_at"`
    Clicks      int         `json:"clicks"`
}

func NewURLModel(dbClient *db.LevelDBClient) *URLModel {
    return &URLModel{
        dbClient: dbClient,
    }
}

func (m *URLModel) Latest() ([]*URLDate, error) {
    var urls []*URLDate

    iter := m.dbClient.DB.NewIterator(nil, nil)
    defer iter.Release()

    for iter.Last(); iter.Valid(); iter.Prev() {
        var urlData URLDate

        if err := json.Unmarshal(iter.Value(), &urlData); err != nil {
            return nil, fmt.Errorf("data parsing failed: %v", err)
        }
        urlData.ShortenedURL = string(iter.Key())

        urls = append(urls, &urlData)

        if len(urls) >= 10 {
            break
        }
    }

    if err := iter.Error(); err != nil {
        return nil, fmt.Errorf(" iterator error: %v", err)
    }

    return urls, nil

}

func (m *URLModel) SaveURL(shortURL, originalURL string) (error) {
    urlDate := URLDate {
        OriginalURL: originalURL,
        CreatedAt: time.Now(),
        Clicks: 0,
    }

    data, err := json.Marshal(urlDate)
    if err != nil {
        return err
    }

    return m.dbClient.Put([]byte(shortURL), data)

}

func (m *URLModel) Get(shortURL string) (*URLDate, error) {
    data, err := m.dbClient.Get([]byte(shortURL))
    if err != nil {
        return nil, err
    }
    if data == nil {
        return nil, nil
    }
    var urlDate URLDate
    err = json.Unmarshal(data, &urlDate)
    if err != nil {
        return nil, err
    }
    return &urlDate, nil
}

func (m *URLModel) IncrementClicks(shortened string) error {
    urlDate, err := m.Get(shortened)
    if err !=nil {
        return err
    }
    if urlDate == nil {
        return fmt.Errorf("URL not found: %s", shortened)
    }

    urlDate.Clicks++

    data, err := json.Marshal(urlDate)
    if err != nil {
        return err
    }

    return m.dbClient.Put([]byte(shortened), data)

}