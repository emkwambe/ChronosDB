package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "net/http"
    "strings"
)

func main() {
    var command, name, role, apiKey string
    
    flag.StringVar(&command, "cmd", "", "Command: create-tenant, create-user, list")
    flag.StringVar(&name, "name", "", "Tenant or user name")
    flag.StringVar(&role, "role", "reader", "User role: admin, writer, reader")
    flag.StringVar(&apiKey, "api-key", "", "Admin API key")
    flag.Parse()
    
    if apiKey == "" {
        fmt.Println("Error: --api-key required")
        return
    }
    
    switch command {
    case "create-tenant":
        createTenant(name, apiKey)
    case "create-user":
        createUser(name, role, apiKey)
    case "list":
        listTenants(apiKey)
    default:
        fmt.Println("Commands: create-tenant, create-user, list")
    }
}

func createTenant(name, apiKey string) {
    body := strings.NewReader(fmt.Sprintf(`{"name":"%s"}`, name))
    req, _ := http.NewRequest("POST", "http://localhost:8080/v1/admin/tenants", body)
    req.Header.Set("X-API-Key", apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    data, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", data)
}

func createUser(name, role, apiKey string) {
    body := strings.NewReader(fmt.Sprintf(`{"username":"%s","role":"%s"}`, name, role))
    req, _ := http.NewRequest("POST", "http://localhost:8080/v1/admin/tenants/default/users", body)
    req.Header.Set("X-API-Key", apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    data, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", data)
}

func listTenants(apiKey string) {
    req, _ := http.NewRequest("GET", "http://localhost:8080/v1/admin/tenants", nil)
    req.Header.Set("X-API-Key", apiKey)
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    data, _ := io.ReadAll(resp.Body)
    fmt.Printf("Tenants: %s\n", data)
}
