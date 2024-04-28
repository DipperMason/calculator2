package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB // Глобальная переменная для базы данных SQLite
var numWorkers int

func main() {
	var err error
	// Инициализация базы данных SQLite
	db, err = InitializeSQLiteDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/receive", receiveData)

	fmt.Println("Receiver server listening on port 8080...")
	http.ListenAndServe(":8080", nil)
}

// Инициализация базы данных SQLite
func InitializeSQLiteDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "my.db")
	if err != nil {
		log.Fatalf("Ошибка при открытии базы данных: %v\n", err)
		return nil, err
	}

	// Создание таблицы expressions, если она не существует
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS expressions ( 
        id INTEGER PRIMARY KEY, 
        expression TEXT, 
        responses TEXT, 
        user TEXT 
    )`)
	if err != nil {
		log.Fatalf("Ошибка при создании таблицы expressions: %v\n", err)
		return nil, err
	}

	// Создание таблицы id_counter, если она не существует
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS id_counter ( 
        id INTEGER 
    )`)
	if err != nil {
		log.Fatalf("Ошибка при создании таблицы id_counter: %v\n", err)
		return nil, err
	}

	// Вставка начального значения в таблицу id_counter, если она пуста
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM id_counter").Scan(&count)
	if err != nil {
		log.Fatalf("Ошибка при проверке таблицы id_counter: %v\n", err)
		return nil, err
	}
	if count == 0 {
		_, err = db.Exec("INSERT INTO id_counter (id) VALUES (0)")
		if err != nil {
			log.Fatalf("Ошибка при вставке начального значения в таблицу id_counter: %v\n", err)
			return nil, err
		}
	}

	return db, nil
}

func receiveData(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received data: %s\n", string(body))
	Start(string(body))
}

func Start(expression string) {
	fmt.Println("Agent started")
	// Проверка выражения на валидность
	re := regexp.MustCompile(`(\d+(\.\d+)?)|([+\-*\/])`)
	if !re.MatchString(expression) {
		fmt.Println("Invalid expression:", expression)
		plusResult(0, false)
	} else {
		fmt.Println("Calculating expression")
		// Решение выражения
		result := start_count(expression)
		fmt.Println(result)
		// Запись результата в базу данных
		plusResult(result, true)
	}
}

func start_count(infixExpression string) float64 {
	postfixExpression := infixToPostfix(infixExpression)
	//использование воркеров для параллельной обработки выражения
	if !(numWorkers > 0) {
		numWorkers = 2
	}
	tasks := make(chan string)
	results := make(chan float64)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(&wg, tasks, results)
	}

	go func() {
		tasks <- postfixExpression
		close(tasks)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()
	var result float64
	for result = range results {
		fmt.Println("Результат вычисления:", result)

	}
	return result
}

func worker(wg *sync.WaitGroup, tasks <-chan string, results chan<- float64) {
	defer wg.Done()
	for task := range tasks {
		result := evaluatePostfix(task)
		results <- result
	}
}

func precedence(op rune) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	}
	return 0
}

func infixToPostfix(infix string) string {
	var postfix []rune
	var stack []rune

	for _, char := range infix {
		if char >= '0' && char <= '9' {
			postfix = append(postfix, char)
		} else if char == '(' {
			stack = append(stack, char)
		} else if char == ')' {
			for len(stack) > 0 && stack[len(stack)-1] != '(' {
				postfix = append(postfix, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			stack = stack[:len(stack)-1]
		} else {
			for len(stack) > 0 && precedence(char) <= precedence(stack[len(stack)-1]) {
				postfix = append(postfix, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, char)
		}
	}

	for len(stack) > 0 {
		postfix = append(postfix, stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}

	return string(postfix)
}

func evaluatePostfix(postfix string) float64 {
	var stack []float64

	for _, char := range postfix {
		if char >= '0' && char <= '9' {
			num, _ := strconv.ParseFloat(string(char), 64)
			stack = append(stack, num)
		} else {
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			timeout := time.After(1 * time.Second) // Задаем ограничение выполнения операций в 1 секунду
			var result float64
			switch char {
			case '+':
				select {
				case <-timeout:
					fmt.Println("Время выполнения операции + истекло")
					return -1
				default:
					result = a + b
				}

			case '-':
				select {
				case <-timeout:
					fmt.Println("Время выполнения операции - истекло")
					return -1
				default:
					result = a - b
				}
			case '*':
				select {
				case <-timeout:
					fmt.Println("Время выполнения операции * истекло")
					return -1
				default:
					result = a * b
				}
			case '/':
				select {
				case <-timeout:
					fmt.Println("Время выполнения операции / истекло")
					return -1
				default:
					result = a / b
				}
			}

			stack = append(stack, result)
		}
	}

	return stack[0]
}

func plusResult(res float64, invalid bool) {
	// Получение текущего значения ID
	var id int
	err := db.QueryRow("SELECT id FROM id_counter").Scan(&id)
	if err != nil {
		log.Fatalf("Ошибка при чтении значения ID: %v\n", err)
		return
	}

	// Увеличение значения ID на 1
	_, err = db.Exec("UPDATE id_counter SET id = id + 1")
	if err != nil {
		log.Fatalf("Ошибка при обновлении значения ID: %v\n", err)
		return
	}

	// Вставка данных в таблицу expressions
	_, err = db.Exec("INSERT INTO expressions (id, expression, responses, user) VALUES (?, ?, ?, ?)",
		id, "expression_placeholder", res,

		"user_placeholder")
	if err != nil {
		log.Fatalf("Ошибка при вставке данных в таблицу expressions: %v\n", err)
		return
	}
}

// func main() {
// 	fmt.Println("aaa")

// 	r := mux.NewRouter()

// 	//Get
// 	r.Handle("/funds/usd/shares", isAuthorized(getusdshar)).Methods("GET")

// 	//Login
// 	r.HandleFunc("/login", login).Methods("POST")

// 	log.Fatal(http.ListenAndServe(":8080", r))

// }

// type User struct {
// 	Username string `json:"username"`
// 	Password string `json:"password"`
// }

// var user = User{
// 	Username: "1",
// 	Password: "1",
// }

// func login(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Access-Control-Origin", "*")
// 	w.Header().Set("Content-Type", "application/json")
// 	var u User
// 	json.NewDecoder(r.Body).Decode(&u)
// 	// fmt.Println("user:", u)
// 	checklogin(u)
// }

// func getusdshar(w http.ResponseWriter, r *http.Request) {
// 	//
// 	w.Header().Set("Access-Control-Origin", "*")
// 	w.Header().Set("Content-Type", "application/json")
// 	var ArrShares = mycurrentfounds("shares")
// 	json.NewEncoder(w).Encode(ArrShares)
// }

// func checklogin(u User) string {
// 	if user.Username != u.Username || user.Password != u.Password {
// 		fmt.Println("Not correct")
// 		err := "error"
// 		return err
// 	}

// 	valtoken, err := Newtoken()
// 	fmt.Println(valtoken)

// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	return valtoken
// }

// func Newtoken() (string, error) {
// 	token := jwt.New(jwt.SigningMethodES256)

// 	claims := token.Claims.(jwt.MapClaims)

// 	claims["exp"] = time.Now().Add(time.Hour * 2).Unix()
// 	claims["user"] = "Joha"
// 	claims["authorized"] = true

// 	tokenstr, err := token.SignedString(mysingkey)

// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	return tokenstr, nil
// }

// func isAuthorized(endpoint func(http.ResponseWriter, *http.Request)) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// 		w.Header().Set("Connection", "close")
// 		defer r.Body.Close()

// 		if r.Header["Token"] != nil {
// 			token, err := jwt.Parse(r.Header["Token"][0], func(token *jwt.Token) (interface{}, error) {
// 				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
// 					return nil, fmt.Errorf("There was an error")
// 				}
// 				return mysingkey, nil
// 			})

// 			if err != nil {
// 				w.WriteHeader(http.StatusForbidden)
// 				w.Header().Add("Content-Type", "application/json")
// 				return
// 			}

// 			if token.Valid {
// 				endpoint(w, r)
// 			}

// 		} else {
// 			fmt.Fprintf(w, "Not Authorized")
// 		}
// 	})
// }

// func mycurrentfounds(fundType string) []Funds {
// 	var amountshares []Funds

// 	db, err := sql.Open("postgress", "postgress://postgress:1234localhost/fin?sslmode+disable")
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	rows, err := db.Query("SELECT = From fundsusd where type = $1", fundType)
// 	for rows.Next() {
// 		f := Funds{}
// 		err = rows.Scan(&f.Id, &f.Vor, &f.Chis)

// 		if err != nil {
// 			log.Fatal(err)
// 		}

// 		amountshares = append(amountshares, f)
// 	}

// 	return amountshares

// }
