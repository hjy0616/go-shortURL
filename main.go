package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"go-shortURL/db"
	"go-shortURL/internals/models"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	urlverifier "github.com/davidmytton/url-verifier"
	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"

	_ "modernc.org/sqlite"
)

type PageData struct {
	BaseURL, Error string
	URLData        []*models.URLDate
}

type App struct {
	urlModel *models.URLModel
	db       *db.LevelDBClient
}

func newApp() (*App, error) {
	//LevelDB 클라이언트 생성
	dbClient, err := db.NewLevelDBClient("./db/database/")
	if err != nil {
		return nil, err
	}

	urlModel := models.NewURLModel(dbClient)

	return &App{
		urlModel: urlModel,
		db:       dbClient,
	}, nil
}

func uniqid(prefix string) string {
	now := time.Now()
	sec := now.Unix()
	usec := now.UnixNano() % 0x100000

	return fmt.Sprintf("%s%08x%05x", prefix, sec, usec)
}

func (a *App) GenerateShortenedURL() string {
	var (
		randomChars   = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0987654321")
		randIntLength = 27
		stringLength  = 32
	)

	str := make([]rune, stringLength)

	for char := range str {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(randIntLength)))
		if err != nil {
			panic(err)
		}

		str[char] = randomChars[nBig.Int64()]
	}

	hash := sha256.Sum256([]byte(uniqid(string(str))))
	encodedString := base64.StdEncoding.EncodeToString(hash[:])

	return encodedString[0:9]
}

func setErrorInFlash(error string, w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "flash-session")
	if err != nil {
		fmt.Println(err.Error())
	}
	session.AddFlash(error, "error")
	session.Save(r, w)
}

var store = sessions.NewCookieStore([]byte("My super secret authentication key"))

func serverError(w http.ResponseWriter, err error) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
	}
	log.Printf("서버 에러: %v", err)
}

var functions = template.FuncMap{
	"formatClicks": formatClicks,
}

func formatClicks(clicks int) string {
	p := message.NewPrinter(language.Korean)
	return p.Sprintf("%v", number.Decimal(clicks))
}

func (a *App) getDefaultRoute(w http.ResponseWriter, r *http.Request) {
	tmplFile := "./templates/default.html"
	tmpl, err := template.New("default.html").Funcs(functions).ParseFiles(tmplFile)
	if err != nil {
		fmt.Println(err.Error())
		serverError(w, err)
		return
	}

	urls, err := a.urlModel.Latest()
	if err != nil {
		fmt.Printf("Could not retrieve all URLs, because %s.\n", err)
		return
	}

	baseURL := "http://" + r.Host + "/"
	pageData := PageData{
		URLData: urls,
		BaseURL: baseURL,
	}

	err = tmpl.Execute(w, pageData)
	if err != nil {
		fmt.Println(err.Error())
		serverError(w, err)
	}
	session, err := store.Get(r, "flash-session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fm := session.Flashes("error")
	if fm != nil {
		if error, ok := fm[0].(string); ok {
			pageData.Error = error
		} else {
			fmt.Printf("Session flash did not contain an error message. Contained %s.\n", fm[0])
		}
	}
	session.Save(r, w)
}

func (a *App) routes() http.Handler {
	router := httprouter.New()
	fileServer := http.FileServer(http.Dir("./static/"))
	router.Handler(http.MethodGet, "/static/*filepath", http.StripPrefix("/static", fileServer))

	router.HandlerFunc(http.MethodGet, "/", a.getDefaultRoute)
	router.HandlerFunc(http.MethodPost, "/", a.shortenURL)
	router.HandlerFunc(http.MethodGet, "/o/:url", a.openShortenedRoute)

	standard := alice.New()

	return standard.Then(router)
}

func main() {
	app, err := newApp()
	if err != nil {
		log.Fatal(err)
	}
	addr := flag.String("addr", ":8080", "HTTP network address")

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	defer app.db.Close()

	srv := &http.Server{
		Addr:     *addr,
		ErrorLog: errorLog,
		Handler:  app.routes(),
	}

	infoLog.Printf("Starting server on %s", *addr)
	err = srv.ListenAndServe()
	errorLog.Fatal(err)
}

func (a *App) shortenURL(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Printf("폼 파싱 에러: %v", err)
		serverError(w, err)
		return
	}

	originalURL := r.PostForm.Get("url")
	if originalURL == "" {
		setErrorInFlash("URL을 입력해주세요.", w, r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	verifier := urlverifier.NewVerifier()
	verifier.EnableHTTPCheck()
	result, err := verifier.Verify(originalURL)
	if err != nil || !result.IsURL || !result.HTTP.Reachable {
		setErrorInFlash("유효하지 않은 URL입니다.", w, r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	shortenedURL := a.GenerateShortenedURL()
	err = a.urlModel.SaveURL(shortenedURL, originalURL)
	if err != nil {
		log.Printf("URL 저장 실패: %v", err)
		setErrorInFlash("URL을 저장할 수 없습니다.", w, r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) openShortenedRoute(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	shortenedURL := params.ByName("url")

	urlData, err := a.urlModel.Get(shortenedURL)
	if err != nil {
		log.Printf("URL 조회 실패: %v", err)
		serverError(w, err)
		return
	}
	if urlData == nil {
		http.NotFound(w, r)
		return
	}

	err = a.urlModel.IncrementClicks(shortenedURL)
	if err != nil {
		log.Printf("방문 수 증가 실패: %v", err)
		serverError(w, err)
		return
	}

	http.Redirect(w, r, urlData.OriginalURL, http.StatusSeeOther)
}