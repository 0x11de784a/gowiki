package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// main page struct - basic element
type Page struct {
	Title       string
	Body        []byte
	DisplayBody template.HTML
}

// regexp for valid paths
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

// regexp for linking like: [page]
var interpageLink, _ = regexp.Compile("\\[[a-zA-Z0-9]+\\]")

// pre-parsed template files
var templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html"))

// globally accessable db object
var db *sql.DB

// persistant saving method
// output: error - nil when no problem occurred
func (p *Page) saveBackup() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

//TESTwise func save clone /w DB implementation
// output: error - nil when no problem occurred
//IDEA: refactor to try UPDATE first, then INSERT
func (p *Page) save() error {
	saveStmt, err := db.Prepare("INSERT pages SET title=?,body=?")
	//checkErr(err) not sufficient, need err as return
	if err != nil {
		return err
	}
	_, err = saveStmt.Exec(p.Title, p.Body)
	if err != nil {
		//1st test suggests: UPDATE here after error check instead of RowsAffected()
		if strings.Contains(err.Error(), "Duplicate entry") {
			updateStmt, err := db.Prepare("UPDATE pages SET body=? WHERE title=?")
			if err != nil {
				return err
			}
			_, err = updateStmt.Exec(p.Body, p.Title)
			if err != nil {
				return err
			}
		}

		return err
	}

	//check error, if entry already exists exec update with body's value
	//QUESTION STILL IS: does it work with affected? or does it throw an error before?
	/*affected, err := res.RowsAffected()
	if affected == 0 {
		//this should be the UPDATE case
		updateStmt, err := db.Prepare("UPDATE pages SET body=? WHERE title=?")
		if err != nil {
			return err
		}
		res, err = updateStmt.Exec(p.Body, p.Title)
		if err != nil {
			return err
		}
	}*/

	return err
	//ToDo: make statements globally prepared. how handle err?
	//ToDo: check name for max of 255 chars (correct?)
}

// load a page from persistant saving
func loadPageBackup(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

//TESTwise func loadPage clone /w DB implementation
func loadPage(title string) (*Page, error) {
	var body []byte
	err := db.QueryRow("SELECT body FROM pages WHERE title=?", title).Scan(&body)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

// redirector for root's URL
func rootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
}

// ???
func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	// replace the linking '[page]' with real html link - just for the view
	escapedBody := []byte(template.HTMLEscapeString(string(p.Body)))
	p.DisplayBody = template.HTML(interpageLink.ReplaceAllFunc(escapedBody, func(input []byte) []byte {
		match := string(input[1 : len(input)-1])
		return []byte("<a href=\"/view/" + match + "\">" + match + "</a>")
	}))
	//---
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// general error check func
func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	// init db, assign to global var
	var err error
	db, err = sql.Open("mysql", "wikiuser:password@/testwiki")
	checkErr(err)

	err = db.Ping()
	checkErr(err)

	// init web app handler
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	http.ListenAndServe(":8080", nil)
}
