package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "root"
	dbname   = "Unvent"
)

// Структуры данных (Сущности)
type Warehouse struct {
	ID      int    `json:"id"`
	Address string `json:"address"`
}

type Products struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Features    map[string]interface{} `json:"features"`
	Weight      float64                `json:"weight"`
	Barcode     string                 `json:"barcode"`
}
type Inventory struct {
	ProductID   int     // ID товара
	WarehouseID int     // ID склада
	Quantity    int     // Количество товара на складе
	Price       float64 // Цена товара на складе
	Discount    float64 // Скидка на товар в процентах (0.0 - 1.0)
}

type Analytics struct {
	WarehouseID int     `json:"warehouse_id"`
	ProductID   int     `json:"product_id"`
	Quantity    int     `json:"quantity"`
	TotalAmount float64 `json:"total_amount"`
}

var dbpool *pgxpool.Pool         // Объявляем dbpool как глобальную переменную
var logger *zap.Logger           // Объявляем logger как глобальную переменную
var products map[string]Products // products хранит товары
var router *mux.Router

func main() {
	// Инициализация логгера
	logger, _ = zap.NewProduction()
	defer logger.Sync()

	// Подключение к базе данных
	dbUrl := "postgres://postgres:root@localhost:5432/Unvent" // Укажите строку подключения
	var err error
	dbpool, err = pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		logger.Fatal("Не удалось подключиться к базе данных", zap.Error(err))
	}
	defer dbpool.Close()

	// m, err := migrate.New(
	// 	"file://db/migrations",
	// 	"cockroachdb://cockroach:@localhost:26257/example?sslmode=disable")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// if err := m.Up(); err != nil {
	// 	log.Fatal(err)
	// }

	products = make(map[string]Products)
	router := mux.NewRouter()
	// Маршруты
	http.HandleFunc("/warehouses", listWarehouses(dbpool, logger))
	http.HandleFunc("/warehouses/create", createWarehouse(dbpool, logger))
	http.HandleFunc("/products", getProductsHandler)
	http.HandleFunc("/products/create", createProductHandler)
	router.HandleFunc("/products/update/{id}", updateProductHandler).Methods("PUT")
	http.HandleFunc("/products/update/{id}", updateProductHandler) // Endpoint обновления
	// Пример HTTP-handler для покупки товара
	http.HandleFunc("/buy", recordAnalytics)

	// Запуск сервера
	fmt.Println("Сервер прослушивает порт 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Основные функции
// WAREHOUSE
func listWarehouses(dbpool *pgxpool.Pool, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Запрос к базе данных
		rows, err := dbpool.Query(context.Background(), "SELECT id, address FROM warehouses")
		if err != nil {
			logger.Error("Не удалось запросить данные о складах", zap.Error(err))
			http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Обработка результатов
		var warehouses []Warehouse
		for rows.Next() {
			var wh Warehouse
			if err := rows.Scan(&wh.ID, &wh.Address); err != nil {
				logger.Error("Не удалось запросить данные о складах", zap.Error(err))
				http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
				return
			}
			warehouses = append(warehouses, wh)
		}

		// Отправка ответа
		fmt.Fprintln(w, "Список складов")
		for _, warehouse := range warehouses {
			fmt.Fprintf(w, "ID: %d, Address: %s\n", warehouse.ID, warehouse.Address)
		}
	}
}

func createWarehouse(dbpool *pgxpool.Pool, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			address := r.FormValue("address")

			_, err := dbpool.Exec(context.Background(), "INSERT INTO warehouses (address) VALUES ($1)", address)
			if err != nil {
				logger.Error("Failed to insert warehouse", zap.Error(err))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			fmt.Fprintln(w, "Склад успешно создан")
		} else {
			http.Error(w, "Метод, который не разрешен", http.StatusMethodNotAllowed)
		}
	}
}

// PRODUCT
// getProductsHandler возвращает список всех товаров
func getProductsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод, который не разрешен", http.StatusMethodNotAllowed)
		return
	}

	products, err := getProducts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// createProductHandler создает новый товар
func createProductHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод, который не разрешен", http.StatusMethodNotAllowed)
		return
	}

	var product Products
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := createProduct(product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Продукт, созданный с ID: %d", id)
}
func updateProductHandler(w http.ResponseWriter, r *http.Request) {
	// Вывод метода запроса
	fmt.Printf("Метод: %s\n", r.Method)

	// Вывод URL
	fmt.Printf("URL: %s\n", r.URL)

	// Вывод заголовков
	fmt.Println("Заголовки:")
	for name, headers := range r.Header {
		for _, h := range headers {
			fmt.Printf("  %v: %v\n", name, h)
		}
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Получаем переменные из URL с помощью gorilla/mux
	vars := mux.Vars(r)
	// Преобразуем vars в JSON
	varsJSON, err := json.Marshal(vars)
	if err != nil {
		http.Error(w, "Ошибка при преобразовании vars в JSON", http.StatusInternalServerError)
		return
	}

	// Выводим JSON в ответе
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(varsJSON))
	productIDStr := vars["id"] // Получаем значение "id" из URL

	if productIDStr == "" {
		http.Error(w, "Отсутствует идентификатор продукта", http.StatusBadRequest)
		return
	}

	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		http.Error(w, "Неверный идентификатор продукта", http.StatusBadRequest)
		return
	}

	// Дальнейшая обработка запроса (декодирование тела запроса, обновление продукта и т.д.)
	fmt.Fprintf(w, "Продукт с ID: %d обновлен\n", productID)
}

// getProducts возвращает список товаров из БД
func getProducts() ([]Products, error) {
	rows, err := dbpool.Query(context.Background(), "SELECT id, name, description, features, weight, barcode FROM products")
	if err != nil {
		return nil, fmt.Errorf("не удалось запросить продукты: %w", err)
	}
	defer rows.Close()

	var products []Products
	for rows.Next() {
		var p Products
		var featuresJSON []byte
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &featuresJSON, &p.Weight, &p.Barcode)
		if err != nil {
			return nil, fmt.Errorf("не удалось найти продукт: %w", err)
		}
		// unmarshal featuresJSON to p.Features
		err = json.Unmarshal(featuresJSON, &p.Features)
		if err != nil {
			return nil, fmt.Errorf("не удалось добавить атрибуты: %w", err)
		}

		products = append(products, p)
	}

	return products, nil
}

// createProduct создает новый товар в БД
func createProduct(p Products) (int, error) {
	featuresJSON, err := json.Marshal(p.Features)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal features: %w", err)
	}
	var id int
	err = dbpool.QueryRow(context.Background(), "INSERT INTO products (name, description, features, weight, barcode) VALUES ($1, $2, $3, $4, $5) RETURNING id", p.Name, p.Description, featuresJSON, p.Weight, p.Barcode).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to insert product: %w", err)
	}

	return id, nil
}

// updateProduct обновляет существующий товар в БД
func updateProduct(p Products) error {
	featuresJSON, err := json.Marshal(p.Features)
	if err != nil {
		return fmt.Errorf("failed to marshal features: %w", err)
	}

	_, err = dbpool.Exec(context.Background(), `
        UPDATE products
        SET description = $1, features = $2
        WHERE id = $3
    `, p.Description, featuresJSON, p.ID)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	return nil
}

// ANALITICS
func recordAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешен только метод POST", http.StatusMethodNotAllowed)
		return
	}

	var analytics Analytics
	err := json.NewDecoder(r.Body).Decode(&analytics)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = dbpool.Exec(context.Background(),
		"INSERT INTO analytics (warehouse_id, product_id, quantity, total_amount) VALUES ($1, $2, $3, $4)",
		analytics.WarehouseID, analytics.ProductID, analytics.Quantity, analytics.TotalAmount)

	if err != nil {
		logger.Error("Не удалось вставить аналитику", zap.Error(err))
		http.Error(w, "Не удалось записать аналитику", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, "Аналитика успешно записана")
}

// package main

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"strconv"

// 	"github.com/jackc/pgx/v5"
// 	"go.uber.org/zap"
// )

// // Product структура товара
// type Product struct {
// 	ID          int                    `json:"id"`
// 	Name        string                 `json:"name"`
// 	Description string                 `json:"description"`
// 	Features    map[string]interface{} `json:"features"`
// 	Weight      float64                `json:"weight"`
// 	Barcode     string                 `json:"barcode"`
// }

// var db *pgx.Conn
// var logger *zap.Logger

// func main() {
// 	// Инициализация логгера (пример, лучше настроить)
// 	logger, _ = zap.NewProduction()
// 	defer logger.Sync()

// 	// Подключение к БД (замените на свои параметры)
// 	conn, err := pgx.Connect(context.Background(), "postgres://postgres:root@localhost:5432/Unvent")
// 	if err != nil {
// 		logger.Fatal("Unable to connect to database", zap.Error(err))
// 	}
// 	defer conn.Close(context.Background())
// 	db = conn

// 	http.HandleFunc("/products", getProductsHandler)
// 	http.HandleFunc("/products/create", createProductHandler)
// 	http.HandleFunc("/products/update", updateProductHandler) // Endpoint обновления

// 	logger.Info("Starting server on :8080")
// 	log.Fatal(http.ListenAndServe(":8080", nil))
// }

// func getProductsHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Метод, который не разрешен", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	products, err := getProducts()
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(products)
// }

// func createProductHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Метод, который не разрешен", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var product Product
// 	err := json.NewDecoder(r.Body).Decode(&product)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	id, err := createProduct(product)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusCreated)
// 	fmt.Fprintf(w, "Product created with ID: %d", id)
// }

// func updateProductHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPut {
// 		http.Error(w, "Метод, который не разрешен", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	productIDStr := r.URL.Query().Get("id")
// 	if productIDStr == "" {
// 		http.Error(w, "Missing product ID", http.StatusBadRequest)
// 		return
// 	}

// 	productID, err := strconv.Atoi(productIDStr)
// 	if err != nil {
// 		http.Error(w, "Invalid product ID", http.StatusBadRequest)
// 		return
// 	}

// 	var product Product
// 	err = json.NewDecoder(r.Body).Decode(&product)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}
// 	product.ID = productID

// 	err = updateProduct(product)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	fmt.Fprint(w, "Product updated successfully")
// }

// // getProducts возвращает список товаров из БД
// func getProducts() ([]Product, error) {
// 	rows, err := db.Query(context.Background(), "SELECT id, name, description, features, weight, barcode FROM products")
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query products: %w", err)
// 	}
// 	defer rows.Close()

// 	var products []Product
// 	for rows.Next() {
// 		var p Product
// 		var featuresJSON []byte
// 		err := rows.Scan(&p.ID, &p.Name, &p.Description, &featuresJSON, &p.Weight, &p.Barcode)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan product: %w", err)
// 		}
// 		// unmarshal featuresJSON to p.Features
// 		err = json.Unmarshal(featuresJSON, &p.Features)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to unmarshal features: %w", err)
// 		}

// 		products = append(products, p)
// 	}

// 	return products, nil
// }

// // createProduct создает новый товар в БД
// func createProduct(p Product) (int, error) {
// 	featuresJSON, err := json.Marshal(p.Features)
// 	if err != nil {
// 		return 0, fmt.Errorf("failed to marshal features: %w", err)
// 	}
// 	var id int
// 	err = db.QueryRow(context.Background(), "INSERT INTO products (name, description, features, weight, barcode) VALUES ($1, $2, $3, $4, $5) RETURNING id", p.Name, p.Description, featuresJSON, p.Weight, p.Barcode).Scan(&id)

// 	if err != nil {
// 		return 0, fmt.Errorf("failed to insert product: %w", err)
// 	}

// 	return id, nil
// }

// // updateProduct обновляет существующий товар в БД
// func updateProduct(p Product) error {
// 	featuresJSON, err := json.Marshal(p.Features)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal features: %w", err)
// 	}

// 	_, err = db.Exec(context.Background(), `
//         UPDATE products
//         SET description = $1, features = $2
//         WHERE id = $3
//     `, p.Description, featuresJSON, p.ID)

// 	if err != nil {
// 		return fmt.Errorf("failed to update product: %w", err)
// 	}

// 	return nil
// }
