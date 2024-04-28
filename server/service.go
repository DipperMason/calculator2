package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	_ "github.com/mattn/go-sqlite3"
)

var New_data string
var TokenString string
var current_user string

const hmacSampleSecret = "super_secret_signature"

var id int
var numWorkers int // Количество воркеров
var additionTimeLimit time.Duration = 1
var subtractionTimeLimit time.Duration = 1
var multiplicationTimeLimit time.Duration = 1
var divisionTimeLimit time.Duration = 1
var mu sync.Mutex

type PageData struct {
	Count1 time.Duration
	Count2 time.Duration
	Count3 time.Duration
	Count4 time.Duration
}

// проверка текущего id
func CheckID(db *sql.DB) int {
	var id int
	err := db.QueryRow("SELECT id FROM id_counter").Scan(&id)
	if err != nil {
		log.Fatalf("Ошибка при чтении значения ID: %v\n", err)
	}
	return id
}

func main() {
	db, err := InitializeSQLiteDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/s", func(w http.ResponseWriter, r *http.Request) {
		FormHandler(w, r, db)
	})

	http.HandleFunc("/n", func(w http.ResponseWriter, r *http.Request) {
		FormHandlerAuth(w, r, db)
	})

	http.HandleFunc("/e", func(w http.ResponseWriter, r *http.Request) {
		FormHandlerReg(w, r, db)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		list_page(w, r)
	})

	http.HandleFunc("/add_page", func(w http.ResponseWriter, r *http.Request) {
		AddPage(w, r, db, current_user)
	})

	http.HandleFunc("/auth_page", func(w http.ResponseWriter, r *http.Request) {
		AuthPage(w, r, db)
	})

	http.HandleFunc("/reg_page", func(w http.ResponseWriter, r *http.Request) {
		reg_page(w, r)
	})

	http.ListenAndServe(":8080", nil)
}

// инициализация базы данных SQLite
func InitializeSQLiteDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "my.db")
	if err != nil {
		log.Fatalf("Ошибка при открытии базы данных: %v\n", err)
		return nil, err
	}
	return db, nil
}

// главная страница
func list_page(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("html/main.html", "html/head.html")
	if err != nil {
		log.Fatalf("Ошибка при инициализации: %v\n", err)
	}
	tmpl.ExecuteTemplate(w, "list_page", nil)
}

// страница регистрации (получения токена)
func reg_page(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("html/registration_jwt.html", "html/head.html")
	if err != nil {
		log.Fatalf("Ошибка при инициализации: %v\n", err)
	}
	tmpl.ExecuteTemplate(w, "reg_page", nil)
}

// страница добавления нового выражения с общим списком из бд (посчитать выражение)
func AddPage(w http.ResponseWriter, r *http.Request, db *sql.DB, currentUser string) {
	tmpl, err := template.ParseFiles("html/calculate.html", "head/head.html")
	if err != nil {
		log.Fatalf("Ошибка при инициализации: %v\n", err)
	}

	rows, err := db.Query("SELECT expression FROM your_table WHERE user = ?", currentUser)
	if err != nil {
		log.Fatalf("Ошибка при выполнении запроса: %v\n", err)
	}
	defer rows.Close()

	var expressions []string
	for rows.Next() {
		var expression string
		err := rows.Scan(&expression)
		if err != nil {
			log.Fatalf("Ошибка при сканировании строки: %v\n", err)
		}
		expressions = append(expressions, expression)
	}

	data := struct {
		Expressions []string
	}{
		Expressions: expressions,
	}

	tmpl.ExecuteTemplate(w, "add_page", data)
}

// аутентификация
func AuthPage(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	tmpl, err := template.ParseFiles("html/auth.html", "html/head.html")
	if err != nil {
		log.Fatalf("Ошибка при инициализации: %v\n", err)
	}

	rows, err := db.Query("SELECT expression FROM your_table")
	if err != nil {
		log.Fatalf("Ошибка при выполнении запроса: %v\n", err)
	}
	defer rows.Close()

	var expressions []string
	for rows.Next() {
		var expression string
		err := rows.Scan(&expression)
		if err != nil {
			log.Fatalf("Ошибка при сканировании строки: %v\n", err)
		}
		expressions = append(expressions, expression)
	}

	data := struct {
		Expressions []string
	}{
		Expressions: expressions,
	}

	tmpl.ExecuteTemplate(w, "auth_page", data)
}

// получаем мат. выражение из формы
func FormHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	data := r.FormValue("data")
	fmt.Printf("Данные: %s\n", data)
	WriteToDatabase(current_user, data, db)

	http.Redirect(w, r, "/add_page", http.StatusMovedPermanently)
}

// форма регистрации
func FormHandlerReg(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	user := r.FormValue("user")
	fmt.Printf("Получено: %s\n", user)
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"name": user,
		"nbf":  now.Unix(),
		"exp":  now.Add(10 * time.Minute).Unix(),
		"iat":  now.Unix(),
	})
	TokenString, _ := token.SignedString([]byte(hmacSampleSecret))
	fmt.Println("Token string:", TokenString)
	WriteToDatabase(TokenString, "", db)

	tmpl, err := template.ParseFiles("html/registration.html", "html/head.html")
	if err != nil {
		log.Fatalf("Ошибка при инициализации: %v\n", err)
	}

	data := struct {
		Variable string
	}{
		Variable: TokenString,
	}

	tmpl.ExecuteTemplate(w, "reg_page", data)
	http.Redirect(w, r, "/reg_page", http.StatusMovedPermanently)
}

// форма аутентификации (если пользователь существует, проверка токена на валидность)
func FormHandlerAuth(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ifErr := ""
	user := ""
	tokenString := r.FormValue("user_token")

	fmt.Printf("Получен токен: %s\n", tokenString)
	found := FindTokenInDatabase(tokenString, db)

	if !found {
		ifErr = "Неверный токен"
	} else {
		tokenFromString, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				fmt.Printf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(hmacSampleSecret), nil
		})

		if err != nil {
			log.Printf("Ошибка: %v\n", err)
			ifErr = "Неверный токен"
		} else {
			if claims, ok := tokenFromString.Claims.(jwt.MapClaims); ok {
				fmt.Println("Имя пользователя: ", claims["name"])
				user = "Добрый день, " + claims["name"].(string)
				current_user = claims["name"].(string)
			} else {
				log.Printf("Ошибка: %v\n", err)
				ifErr = "Неверный токен"
			}
		}
	}

	if ifErr != "" {
		user = ifErr
	}

	tmpl, err := template.ParseFiles("html/auth.html", "html/head.html")
	if err != nil {
		log.Printf("Ошибка при инициализации: %v\n", err)
	}

	data := struct {
		Var string
	}{
		Var: user,
	}

	tmpl.ExecuteTemplate(w, "auth_page", data)
	http.Redirect(w, r, "/auth_page", http.StatusMovedPermanently)
}

// проверка токена
func FindTokenInDatabase(tokenCheck string, db *sql.DB) bool {
	id := CheckID(db)
	for i := 1; i <= id; i++ {
		var user string
		err := db.QueryRow("SELECT user FROM expressions WHERE id = ? AND user = ?", i, tokenCheck).Scan(&user)
		if err == nil {
			return true
		}
	}
	return false
}

// запись выражения в бд
func WriteToDatabase(user string, data string, db *sql.DB) {
	id := CheckID(db)
	id++

	_, err := db.Exec("UPDATE id_counter SET id = ?", id)
	if err != nil {
		log.Fatalf("Ошибка при обновлении значения ID: %v\n", err)
	}

	_, err = db.Exec("INSERT INTO expressions (id, expression, responses, user) VALUES (?, ?, 'new', ?)", id, data, user)
	if err != nil {
		log.Fatalf("Ошибка при записи данных в базу данных: %v\n", err)
	}

	log.Println("Данные успешно записаны в базу данных!")

	if data != "" {
		newData := []byte(data) // Данные для отправки агентам
		resp, err := http.Post("http://localhost:8081/receive", "text/plain", bytes.NewBuffer(newData))
		if err != nil {
			fmt.Println("Ошибка при отправке данных:", err)
			return
		}
		defer resp.Body.Close()
		fmt.Println("Данные успешно отправлены")
	}
}
