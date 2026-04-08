package main

import (
    "encoding/json"
    "fmt"
    "math/rand"
    "time"
)

// SyntheticDataGenerator generates realistic test data
type SyntheticDataGenerator struct {
    customers []Customer
    products  []Product
    orders    []Order
}

type Customer struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Age       int       `json:"age"`
    City      string    `json:"city"`
    CreatedAt time.Time `json:"created_at"`
}

type Product struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    Category    string  `json:"category"`
    Price       float64 `json:"price"`
    Stock       int     `json:"stock"`
}

type Order struct {
    ID         string    `json:"id"`
    CustomerID string    `json:"customer_id"`
    ProductID  string    `json:"product_id"`
    Quantity   int       `json:"quantity"`
    Total      float64   `json:"total"`
    Status     string    `json:"status"`
    OrderDate  time.Time `json:"order_date"`
}

// GenerateCustomers creates synthetic customer data
func (g *SyntheticDataGenerator) GenerateCustomers(n int) []Customer {
    firstNames := []string{"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda", "William", "Elizabeth"}
    lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez"}
    cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "Austin"}
    
    customers := make([]Customer, n)
    for i := 0; i < n; i++ {
        customers[i] = Customer{
            ID:        fmt.Sprintf("cust_%d", i+1),
            Name:      fmt.Sprintf("%s %s", firstNames[rand.Intn(len(firstNames))], lastNames[rand.Intn(len(lastNames))]),
            Email:     fmt.Sprintf("user%d@example.com", i+1),
            Age:       18 + rand.Intn(60),
            City:      cities[rand.Intn(len(cities))],
            CreatedAt: time.Now().AddDate(0, -rand.Intn(24), -rand.Intn(30)),
        }
    }
    g.customers = customers
    return customers
}

// GenerateProducts creates synthetic product data
func (g *SyntheticDataGenerator) GenerateProducts(n int) []Product {
    categories := []string{"Electronics", "Clothing", "Books", "Home", "Sports", "Toys", "Food", "Beauty"}
    productNames := []string{"Laptop", "Smartphone", "Headphones", "T-Shirt", "Jeans", "Novel", "Cookbook", "Lamp", "Chair", "Table"}
    
    products := make([]Product, n)
    for i := 0; i < n; i++ {
        products[i] = Product{
            ID:       fmt.Sprintf("prod_%d", i+1),
            Name:     fmt.Sprintf("%s %d", productNames[rand.Intn(len(productNames))], i+1),
            Category: categories[rand.Intn(len(categories))],
            Price:    float64(10+rand.Intn(990)) + float64(rand.Intn(99))/100,
            Stock:    10 + rand.Intn(190),
        }
    }
    g.products = products
    return products
}

// GenerateOrders creates synthetic order data with temporal patterns
func (g *SyntheticDataGenerator) GenerateOrders(n int) []Order {
    statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
    
    orders := make([]Order, n)
    for i := 0; i < n; i++ {
        customer := g.customers[rand.Intn(len(g.customers))]
        product := g.products[rand.Intn(len(g.products))]
        quantity := 1 + rand.Intn(5)
        
        orders[i] = Order{
            ID:         fmt.Sprintf("order_%d", i+1),
            CustomerID: customer.ID,
            ProductID:  product.ID,
            Quantity:   quantity,
            Total:      product.Price * float64(quantity),
            Status:     statuses[rand.Intn(len(statuses))],
            OrderDate:  time.Now().AddDate(0, 0, -rand.Intn(90)),
        }
    }
    g.orders = orders
    return orders
}

// GenerateTimeSeries creates temporal data with trends
func GenerateTimeSeries(baseValue float64, days int, trend float64, seasonality bool) []struct {
    Timestamp int64
    Value     float64
} {
    data := make([]struct {
        Timestamp int64
        Value     float64
    }, days)
    
    for i := 0; i < days; i++ {
        timestamp := time.Now().AddDate(0, 0, -days+i).UnixMicro()
        value := baseValue + float64(i)*trend
        
        if seasonality {
            value += 10 * float64(rand.Intn(20)-10) // Add noise
        }
        
        data[i] = struct {
            Timestamp int64
            Value     float64
        }{Timestamp: timestamp, Value: value}
    }
    return data
}

func main() {
    rand.Seed(time.Now().UnixNano())
    generator := &SyntheticDataGenerator{}
    
    // Generate data
    fmt.Println("Generating synthetic data...")
    customers := generator.GenerateCustomers(100)
    products := generator.GenerateProducts(50)
    orders := generator.GenerateOrders(500)
    
    // Save to JSON files
    saveJSON("test/data/customers.json", customers)
    saveJSON("test/data/products.json", products)
    saveJSON("test/data/orders.json", orders)
    
    // Generate time series data
    fmt.Println("Generating time series data...")
    sensorData := GenerateTimeSeries(100.0, 365, 0.1, true)
    saveJSON("test/data/sensor_data.json", sensorData)
    
    fmt.Printf("Generated: %d customers, %d products, %d orders, %d sensor readings\n", 
        len(customers), len(products), len(orders), len(sensorData))
}

func saveJSON(filename string, data interface{}) {
    file, _ := json.MarshalIndent(data, "", "  ")
    fmt.Printf("Saved: %s\n", filename)
    _ = file // In real implementation, write to file
}
