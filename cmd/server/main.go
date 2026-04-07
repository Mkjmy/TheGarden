package main

import (
	"crypto/sha256"
	"fmt"
	"garden-onion/internal/database"
	"garden-onion/internal/models"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"
)

type PageData struct {
	Threads      []models.Thread
	Post         *models.Thread
	Responses    []models.Thread
	TagStats     []models.TagStat
	Body         template.HTML
	AuthorName   string
	Votes        map[int]int
	CurrentTab   string
	PendingCount int
	ParentID     int
	ParentAuthor string
	IsReadOnly   bool
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "GARDEN")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline';")
		next.ServeHTTP(w, r)
	})
}

func main() {
	database.InitDB()
	
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/garden", gardenHandler)
	mux.HandleFunc("/post", postHandler)
	mux.HandleFunc("/thread", threadHandler)
	mux.HandleFunc("/comment", commentHandler)
	mux.HandleFunc("/vote", voteHandler)
	mux.HandleFunc("/source", sourceHandler)
	mux.HandleFunc("/principles", principlesHandler)
	mux.HandleFunc("/nap", napHandler)

	port := ":8080"
	log.Printf("GARDEN server is starting at http://localhost%s...", port)
	if err := http.ListenAndServe(port, secureHeaders(mux)); err != nil {
		log.Fatal(err)
	}
}

func getVisitorID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("visitor_id")
	if err == nil { return cookie.Value }
	id := uuid.New().String()
	http.SetCookie(w, &http.Cookie{Name: "visitor_id", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return id
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" { http.NotFound(w, r); return }
	renderTemplate(w, "index.html", PageData{})
}

func gardenHandler(w http.ResponseWriter, r *http.Request) {
	isRO := models.IsDeadMansSwitchActive()
	vid := getVisitorID(w, r)
	tab := r.URL.Query().Get("tab")
	if tab == "" { tab = "feed" }
	var threads []models.Thread
	if tab == "top" { threads, _ = models.GetTopThreads(10) } else { threads, _ = models.GetAllThreads(tab) }
	votes, _ := models.GetVisitorVotes(vid)
	if votes == nil { votes = make(map[int]int) }
	pending := models.GetPendingCount()
	renderTemplate(w, "garden.html", PageData{Threads: threads, Votes: votes, CurrentTab: tab, PendingCount: pending, IsReadOnly: isRO})
}

func threadHandler(w http.ResponseWriter, r *http.Request) {
	isRO := models.IsDeadMansSwitchActive()
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	thread, err := models.GetThreadByID(id)
	if err != nil { http.NotFound(w, r); return }
	vid := getVisitorID(w, r)
	votes, _ := models.GetVisitorVotes(vid)
	if votes == nil { votes = make(map[int]int) }
	tagStats, _ := models.GetThreadTagStats(id)
	responses, _ := models.GetResponses(id)
	unsafeHTML := markdown.ToHTML([]byte(thread.Content), nil, nil)
	safeHTML := bluemonday.UGCPolicy().SanitizeBytes(unsafeHTML)
	renderTemplate(w, "thread.html", PageData{Post: thread, Responses: responses, TagStats: tagStats, Body: template.HTML(safeHTML), Votes: votes, IsReadOnly: isRO})
}

func voteHandler(w http.ResponseWriter, r *http.Request) {
	if models.IsDeadMansSwitchActive() { http.Error(w, "The Garden is in Read-Only mode.", 403); return }
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	vtype := r.URL.Query().Get("type")
	vid := getVisitorID(w, r)
	delta := 0
	if vtype == "up" { delta = 1 } else if vtype == "down" { delta = -1 }
	if delta != 0 { _ = models.ToggleVote(id, vid, delta) }
	ref := r.Header.Get("Referer")
	if ref == "" { ref = "/garden" }
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func commentHandler(w http.ResponseWriter, r *http.Request) {
	if models.IsDeadMansSwitchActive() { http.Error(w, "The Garden is in Read-Only mode.", 403); return }
	if r.Method != http.MethodPost { http.Redirect(w, r, "/garden", http.StatusSeeOther); return }
	tid, _ := strconv.Atoi(r.FormValue("thread_id"))
	stag := r.FormValue("suggested_tag")
	vid := getVisitorID(w, r)
	if stag != "" { _ = models.SuggestTag(tid, vid, stag) }
	http.Redirect(w, r, "/thread?id="+strconv.Itoa(tid), 303)
}

func sourceHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" { http.NotFound(w, r); return }
	threads, _ := models.GetThreadsByAuthor(name)
	renderTemplate(w, "author.html", PageData{Threads: threads, AuthorName: name})
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	isRO := models.IsDeadMansSwitchActive()
	if r.Method == http.MethodGet {
		pid, _ := strconv.Atoi(r.URL.Query().Get("parent_id"))
		pan := r.URL.Query().Get("parent_author")
		renderTemplate(w, "post.html", PageData{ParentID: pid, ParentAuthor: pan, IsReadOnly: isRO})
		return
	}
	if r.Method == http.MethodPost {
		if isRO { http.Error(w, "The Garden is in Read-Only mode.", 403); return }
		t, c, an, ds := r.FormValue("title"), r.FormValue("content"), r.FormValue("author_name"), r.FormValue("delay_hours")
		nonce := r.FormValue("pow_nonce")

		h := sha256.New()
		h.Write([]byte(t + c + nonce))
		hashHex := fmt.Sprintf("%x", h.Sum(nil))
		
		if hashHex[:4] != "0000" {
			log.Printf("PoW Failed! Expected 0000..., got %s", hashHex)
			http.Error(w, "Proof-of-Work verification failed. Please try again.", 400)
			return
		}

		pid, _ := strconv.Atoi(r.FormValue("parent_id"))
		pan := r.FormValue("parent_author")
		delay, _ := strconv.Atoi(ds)
		
		if an == "" { an = "@anonymous" }

		p := bluemonday.StrictPolicy()
		cleanTitle := p.Sanitize(t)
		cleanContent := p.Sanitize(c)
		now := time.Now().UTC()
		var pa time.Time
		if delay <= 0 { pa = now } else { pa = now.Add(time.Duration(delay) * time.Hour) }

		err := models.SaveThreadWithSchedule(cleanTitle, cleanContent, an, "feed", pid, pan, pa)
		if err != nil {
			log.Printf("SAVE ERROR: %v", err)
			http.Error(w, "Could not save signal to database", 500)
			return
		}
		
		if pid > 0 { http.Redirect(w, r, "/thread?id="+strconv.Itoa(pid), 303) } else { http.Redirect(w, r, "/garden", 303) }
	}
}

func principlesHandler(w http.ResponseWriter, r *http.Request) { renderTemplate(w, "principles.html", PageData{}) }
func napHandler(w http.ResponseWriter, r *http.Request) { renderTemplate(w, "nap.html", PageData{}) }

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmplPath := filepath.Join("web", "templates", tmplName)
	funcMap := template.FuncMap{
		"until": func(n int) []int { var res []int; for i := 0; i < n; i++ { res = append(res, i) }; return res },
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 { dict[values[i].(string)] = values[i+1] }
			return dict, nil
		},
	}
	tmpl, err := template.New(tmplName).Funcs(funcMap).ParseFiles(tmplPath)
	if err != nil { log.Printf("Error rendering template %s: %v", tmplName, err); http.Error(w, "Server Error.", 500); return }
	tmpl.Execute(w, data)
}
