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
	ID          int
	ProductID   int     // ID товара
	WarehouseID int     // ID склада
	Quantity    int     // Количество товара на складе
	Price       float64 // Цена товара на складе
	Discount    float64 // Скидка на товар в процентах (0.0 - 1.0)
}

type Analytics struct {
	ProductName string  `json:"product_name"`
	TotalSold   int     `json:"total_sold"`
	TotalSum    float64 `json:"total_sum"`
}

type WarehouseRevenue struct {
	WarehouseID  int     `json:"warehouse_id"`
	Address      string  `json:"address"`
	TotalRevenue float64 `json:"total_revenue"`
}

var dbpool *pgxpool.Pool
var logger *zap.Logger
var products map[string]Products
var router *mux.Router

func main() {
	// Конфигурация энкодера для JSON формата.
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

	products = make(map[string]Products)
	router := mux.NewRouter()
	// Маршруты
	router.HandleFunc("/warehouses", listWarehouses(dbpool, logger))
	router.HandleFunc("/warehouses/create", createWarehouseHandler).Methods("POST")
	router.HandleFunc("/products", getProductsHandler)
	router.HandleFunc("/products/create", createProductHandler)
	router.HandleFunc("/products/update/{id:[0-9]+}", updateProductHandler).Methods("PUT")
	router.HandleFunc("/inventory/create", createInventory).Methods("POST")
	router.HandleFunc("/inventory/{id:[0-9]+}", updateInventory).Methods("PUT")
	router.HandleFunc("/inventory/discount", createDiscount).Methods("POST")
	router.HandleFunc("/inventory/warehouse/{warehouse:[0-9]+}", getInventoryByWarehouse).Methods("GET")
	router.HandleFunc("/inventory/{id:[0-9]+}", getInventoryItem).Methods("GET")
	router.HandleFunc("/inventory/summary", getInventorySummary).Methods("POST")
	router.HandleFunc("/inventory/purchase", purchaseItems).Methods("POST")
	router.HandleFunc("/analytics", updateAnalytics).Methods("PUT")
	router.HandleFunc("/analytics/warehouse/{warehouse_id}", getAnalyticsByWarehouse).Methods("GET")
	router.HandleFunc("/top-warehouses", getTopWarehouses).Methods("GET")
	http.Handle("/", router)

	// Запуск сервера
	fmt.Println("Сервер прослушивает порт 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
	logger.Sync()
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

func createWarehouseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	var warehouse Warehouse
	if err := json.NewDecoder(r.Body).Decode(&warehouse); err != nil {
		http.Error(w, "Неверный запрос", http.StatusBadRequest)
		return
	}

	// Проверяем, заполнен ли адрес
	if warehouse.Address == "" {
		http.Error(w, "Адрес склада обязателен", http.StatusBadRequest)
		return
	}

	// Сохраняем склад в базе данных
	err := createWarehouseInDB(dbpool, warehouse.Address)
	if err != nil {
		logger.Error("Не удалось вставить склад", zap.Error(err))
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	// Успешный ответ
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(warehouse)
}

func createWarehouseInDB(dbpool *pgxpool.Pool, address string) error {
	_, err := dbpool.Exec(context.Background(), "INSERT INTO warehouses (address) VALUES ($1)", address)
	return err
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
func updateProduct(w http.ResponseWriter, r *http.Request) {
	var p Products
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := dbpool.Exec(context.Background(), "UPDATE products SET description = $1, attributes = $2 WHERE id = $3",
		p.Description, p.Features, p.ID)
	if err != nil {
		logger.Error("Update failed", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// ANALITICS
func updateAnalytics(w http.ResponseWriter, r *http.Request) {
	var sale Inventory
	if err := json.NewDecoder(r.Body).Decode(&sale); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := dbpool.Exec(context.Background(), "INSERT INTO analytics (warehouse_id, product_id, quantity, total_amount) VALUES ($1, $2, $3, $4)",
		sale.WarehouseID, sale.ProductID, sale.Quantity, sale.Discount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getAnalyticsByWarehouse(w http.ResponseWriter, r *http.Request) {
	warehouseID := mux.Vars(r)["warehouse_id"]
	rows, err := dbpool.Query(context.Background(), "SELECT p.name, SUM(a.quantity), SUM(a.total_amount) FROM analytics a JOIN products p ON a.product_id = p.id WHERE a.warehouse_id = $1 GROUP BY p.name", warehouseID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var analytics []Analytics
	for rows.Next() {
		var a Analytics
		if err := rows.Scan(&a.ProductName, &a.TotalSold, &a.TotalSum); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		analytics = append(analytics, a)
	}

	json.NewEncoder(w).Encode(analytics)
}

func getTopWarehouses(w http.ResponseWriter, r *http.Request) {
	rows, err := dbpool.Query(context.Background(), "SELECT w.id, w.address, SUM(a.total_amount) FROM analytics a JOIN warehouses w ON a.warehouse_id = w.id GROUP BY w.id ORDER BY SUM(a.total_amount) DESC LIMIT 10")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var revenues []WarehouseRevenue
	for rows.Next() {
		var r WarehouseRevenue
		if err := rows.Scan(&r.WarehouseID, &r.Address, &r.TotalRevenue); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		revenues = append(revenues, r)
	}

	json.NewEncoder(w).Encode(revenues)
}

// INVENTORY
func createInventory(w http.ResponseWriter, r *http.Request) {
	var inv Inventory
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := dbpool.Exec(context.Background(), "INSERT INTO inventory (productid, warehouseid, quantity, price, discount) VALUES ($1, $2, $3, $4, $5)", inv.ProductID, inv.WarehouseID, inv.Quantity, inv.Price, inv.Discount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func updateInventory(w http.ResponseWriter, r *http.Request) {
	var inv Inventory
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := dbpool.Exec(context.Background(), "UPDATE inventory SET quantity = quantity + $1 WHERE id = $2", inv.Quantity, inv.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func createDiscount(w http.ResponseWriter, r *http.Request) {
	var discount struct {
		ProductIDs []int   `json:"product_ids"`
		Discount   float64 `json:"discount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&discount); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := dbpool.Exec(context.Background(), "UPDATE inventory SET discount = $1 WHERE productid = ANY($2)", discount.Discount, discount.ProductIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func getInventoryByWarehouse(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	warehouseID := vars["warehouseID"]
	rows, err := dbpool.Query(context.Background(), "SELECT id, productid, price, discount FROM inventory WHERE warehouseid = $1", warehouseID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var inventories []Inventory
	for rows.Next() {
		var inv Inventory
		if err := rows.Scan(&inv.ID, &inv.ProductID, &inv.Price, &inv.Discount); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		inventories = append(inventories, inv)
	}
	json.NewEncoder(w).Encode(inventories)
}

func getInventoryItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var inv Inventory
	err := dbpool.QueryRow(context.Background(), "SELECT id, productid, quantity, price, discount FROM inventory WHERE id = $1", id).Scan(&inv.ID, &inv.ProductID, &inv.Quantity, &inv.Price, &inv.Discount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(inv)
}

func getInventorySummary(w http.ResponseWriter, r *http.Request) {
	var summary struct {
		WarehouseID int             `json:"warehouseid"`
		Items       []InventoryItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&summary); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var total float64
	for _, item := range summary.Items {
		var price float64
		err := dbpool.QueryRow(context.Background(), "SELECT price FROM inventory WHERE warehouseid = $1 AND product_id = $2", summary.WarehouseID, item.ProductID).Scan(&price)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		total += price * float64(item.Quantity)
	}
	json.NewEncoder(w).Encode(map[string]float64{"total": total})
}

func purchaseItems(w http.ResponseWriter, r *http.Request) {
	var purchase struct {
		WarehouseID int             `json:"warehouseid"`
		Items       []InventoryItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&purchase); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, item := range purchase.Items {
		var quantity int
		err := dbpool.QueryRow(context.Background(), "SELECT quantity FROM inventory WHERE warehouseid = $1 AND productid = $2", purchase.WarehouseID, item.ProductID).Scan(&quantity)
		if err != nil || quantity < item.Quantity {
			http.Error(w, "Insufficient quantity", http.StatusBadRequest)
			return
		}
		_, err = dbpool.Exec(context.Background(), "UPDATE inventory SET quantity = quantity - $1 WHERE warehouseid = $2 AND productid = $3", item.Quantity, purchase.WarehouseID, item.ProductID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

type InventoryItem struct {
	ProductID int `json:"productid"`
	Quantity  int `json:"quantity"`
}
