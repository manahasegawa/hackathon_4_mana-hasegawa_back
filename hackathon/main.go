package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/oklog/ulid/v2"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// アイテムの型構造を定義
type ItemResForHTTPGet struct {
	Id          string `json:"id"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
	Time        string `json:"time"`
	Category    string `json:"category"`
	Tag         string `json:"tag"`
}

// ① GoプログラムからMySQLへ接続
var db *sql.DB

func init() {
	// ①-1
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("fail: loadfailed, %v\n", err)
	}

	mysqlUser := os.Getenv("MYSQL_USER")
	mysqlUserPwd := os.Getenv("MYSQL_PWD")
	mysqlHost := os.Getenv("MYSQL_HOST")
	mysqlDatabase := os.Getenv("MYSQL_DATABASE")

	// ①-2
	_db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@%s/%s", mysqlUser, mysqlUserPwd, mysqlHost, mysqlDatabase))
	if err != nil {
		log.Fatalf("fail: sql.Open, %v\n", err)
	}
	// ①-3
	if err := _db.Ping(); err != nil {
		log.Fatalf("fail: _db.Ping, %v\n", err)
	}
	db = _db
}

func handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	//この行を入れたらエラーが消えた
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	//リクエストされたらアイテムのレコードをJSON形式で返す
	case http.MethodGet:

		//GETのクエリ文を定義
		rows, err := db.Query(
			"SELECT e.id, e.title, e.explanation, e.time,e.category, f.tag FROM ( SELECT  a.*, b.category FROM item AS a JOIN category AS b ON a.category_id = b.id) AS e JOIN ( SELECT c.*, d.curriculum AS tag FROM itemtocurriculum AS c JOIN curriculum AS d ON c.curriculum_id = d.id ) AS f ON e.id = f.item_id;")
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// アイテムをリストに格納する
		items := make([]ItemResForHTTPGet, 0)
		for rows.Next() {
			var u ItemResForHTTPGet
			if err := rows.Scan(&u.Id, &u.Title, &u.Explanation, &u.Time, &u.Category, &u.Tag); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			items = append(items, u)
		}
		bytes, err := json.Marshal(items)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:

		t := time.Now()
		entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
		id := ulid.MustNew(ulid.Timestamp(t), entropy)
		itemtocid := ulid.MustNew(ulid.Timestamp(t), entropy)

		var postItem struct {
			Title         string `json:"title"`
			Explanation   string `json:"explanation"`
			Time          string `json:"time"`
			Category_id   int    `json:"category_id"`
			Tag           string `json:"tag"`
			Curriculum_id int    `json:"curriculum_id"`
		}

		if postItem.Title == "" || postItem.Explanation == "" || postItem.Tag == "" {
			w.WriteHeader(http.StatusBadRequest)
		}

		//データベースにinsert
		insert, err := db.Prepare(
			"BEGIN;INSERT INTO item(id,title,category_id,explanation,time) VALUES (?,?,?,?,CURRENT_TIMESTAMP); INSERT INTO itemtocurriculum(id, item_id, curriculum_id) VALUES (?,?,?); COMMIT;")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		} else {
			w.WriteHeader(http.StatusOK)
			response := map[string]string{"id": id.String()}
			bytes, err := json.Marshal(response)
			if err != nil {
				log.Printf("fail: json.Marshal, %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(bytes)

		}
		insert.Exec(id, postItem.Title, postItem.Category_id, postItem.Explanation, itemtocid, id, postItem.Curriculum_id)

	//case http.MethodDelete:

	//case http.MethodPatch:

	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func main() {
	// ② /userでリクエストされたらnameパラメーターと一致する名前を持つレコードをJSON形式で返す
	http.HandleFunc("/", handler)

	// ③ Ctrl+CでHTTPサーバー停止時にDBをクローズする
	closeDBWithSysCall()

	// 8000番ポートでリクエストを待ち受ける
	log.Println("Listening...")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func closeDBWithSysCall() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sig
		log.Printf("received syscall, %v", s)

		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
		log.Printf("success: db.Close()")
		os.Exit(0)
	}()
}
