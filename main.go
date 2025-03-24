// package main

// import (
// 	"fmt"
// 	"net/http"
// )

// // Обработчик для корневого пути "/"
// func handler(w http.ResponseWriter, r *http.Request) {
// 	fmt.Fprintf(w, "Привет, мир!") // Отправляем "Привет, мир!" в HTTP-ответе
// }

// func main() {
// 	http.HandleFunc("/", handler) // Регистрируем обработчик для корневого пути

// 	fmt.Println("Сервер запущен на порту 8080")
// 	err := http.ListenAndServe(":8080", nil) // Запускаем HTTP-сервер на порту 8080
// 	if err != nil {
// 		fmt.Println("Ошибка запуска сервера:", err)
// 	}
// }

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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
type Warehouses struct {
	ID      int    `json:"id"`
	Address string `json:"address"`
}

type Product struct {
	ID          int
	Name        string
	Description string
	Attributes  map[string]interface{} // Характеристики товара: ключ - название, значение - значение.
	Weight      float64                // Вес одной единицы товара в кг.
	Barcode     string                 // Штрих-код товара.
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

// Основные функции
// WAREHOUSE
func listWarehouses(dbpool *pgxpool.Pool, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Запрос к базе данных
		rows, err := dbpool.Query(context.Background(), "SELECT id, address FROM warehouses")
		if err != nil {
			logger.Error("Failed to query warehouses", zap.Error(err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Обработка результатов
		var warehouses []Warehouse
		for rows.Next() {
			var wh Warehouse
			if err := rows.Scan(&wh.ID, &wh.Address); err != nil {
				logger.Error("Failed to scan warehouse", zap.Error(err))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

// PRODUCT
// getProductsHandler возвращает список всех товаров
func getProductsHandler(w http.ResponseWriter, r *http.Request) {
	productList := make([]Product, 0, len(products))
	for _, product := range products {
		productList = append(productList, product)
	}

	json.NewEncoder(w).Encode(productList)
}

// createProductHandler создает новый товар
func createProductHandler(w http.ResponseWriter, r *http.Request) {
	var product Product
	_ = json.NewDecoder(r.Body).Decode(&product)

	product.ID = uuid.New().String() // генерируем UUID
	products[product.ID] = product

	json.NewEncoder(w).Encode(product)
}

// ANALITICS
func recordAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
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
		logger.Error("Failed to insert analytics", zap.Error(err))
		http.Error(w, "Failed to record analytics", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, "Analytics recorded successfully")
}

func main() {
	// Устанавливаем значение переменной окружения DATABASE_URL
	err := os.Getenv("DATABASE_URL", "postgres://postgres:root@localhost/Unvent")[1]
	if err != nil {
		fmt.Println("Error setting environment variable:", err)[1]
	}
	// Проверяем, что переменная была установлена
	dbUrl := os.Getenv("DATABASE_URL")[1]
	fmt.Println("Database URL:", dbUrl)[1]

	// Подключение к базе данных
	dbpool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Fatal("Unable to connect to database", zap.Error(err))
	}
	defer dbpool.Close()

	// Инициализация логгера
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	// Маршруты
	http.HandleFunc("/products", getProductsHandler)
	http.HandleFunc("/stocks", getStocksHandler)
	http.HandleFunc("/add_stock", addStockHandler)
	http.HandleFunc("/sale", saleHandler)
	http.HandleFunc("/warehouses", listWarehouses(dbpool, logger))
	http.HandleFunc("/warehouses/create", createWarehouse(dbpool, logger))
	router.HandleFunc("/products", getProductsHandler).Methods("GET")
	router.HandleFunc("/products", createProductHandler).Methods("POST")

	// Запуск сервера
	fmt.Println("Server listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getProductsHandler(w http.ResponseWriter, r *http.Request) {}
func getStocksHandler(w http.ResponseWriter, r *http.Request)   {}
func addStockHandler(w http.ResponseWriter, r *http.Request)    {}
func saleHandler(w http.ResponseWriter, r *http.Request)        {}
