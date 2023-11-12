package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
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

	Curriculum string `json:"curriculum"`
}

// ① GoプログラムからMySQLへ接続
var db *sql.DB

func init() {
	/*
		// ①-1
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatalf("fail: loadfailed, %v\n", err)
		}

	*/

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

	w.Header().Set("Access-Control-Allow-Origin", "https://hackathon-4-mana-hasegawa-fro-git-c66531-manahasegawas-projects.vercel.app/")
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

			"SELECT i.title, i.explanation ,i.time, ca.category ,cu.curriculum  FROM item AS i JOIN category AS ca ON i.category_id = ca.id JOIN curriculum AS cu ON i.curriculum_id = cu.id;")

		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// アイテムをリストに格納する
		items := make([]ItemResForHTTPGet, 0)
		for rows.Next() {
			var u ItemResForHTTPGet

			if err := rows.Scan(&u.Title, &u.Explanation, &u.Time, &u.Category, &u.Curriculum); err != nil {

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

		type postData struct {
			Title         string `json:"title"`
			Category_id   int    `json:"category"`
			Curriculum_id int    `json:"curriculum"`
			Explanation   string `json:"explanation"`
		}

		t := time.Now()
		entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
		id := ulid.MustNew(ulid.Timestamp(t), entropy)

		// HTTPリクエストボディからJSONデータを読み取る
		decoder := json.NewDecoder(r.Body)
		var readData postData
		if err := decoder.Decode(&readData); err != nil {

			log.Printf("fail: json.Decode, %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if readData.Title == "" || readData.Explanation == "" {

			w.WriteHeader(http.StatusBadRequest)
		}

		//データベースにinsert

		_, err := db.Exec(
			"INSERT INTO item(id,title,category_id,explanation,curriculum_id,time) VALUES (?,?,?,?,?,CURRENT_TIMESTAMP);",
			id.String(), readData.Title, readData.Category_id, readData.Explanation, readData.Curriculum_id)

		if err != nil {
			log.Printf("insert err")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

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
		port = "8080"
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
