package multitenancy

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "sync"
)

// Tenant represents a database tenant
type Tenant struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    APIKey      string   `json:"api_key"`
    Databases   []string `json:"databases"`
    Quota       Quota    `json:"quota"`
    CreatedAt   int64    `json:"created_at"`
}

// Quota defines tenant limits
type Quota struct {
    MaxNodes     int `json:"max_nodes"`
    MaxEdges     int `json:"max_edges"`
    MaxQueries   int `json:"max_queries_per_minute"`
    StorageBytes int `json:"storage_bytes"`
}

// Role defines user roles
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleWriter Role = "writer"
    RoleReader Role = "reader"
)

// User represents a tenant user
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    TenantID string `json:"tenant_id"`
    Role     Role   `json:"role"`
    APIKey   string `json:"api_key"`
}

// TenantManager handles multi-tenancy
type TenantManager struct {
    tenants map[string]*Tenant
    users   map[string]*User
    mu      sync.RWMutex
}

// NewTenantManager creates a new tenant manager
func NewTenantManager() *TenantManager {
    return &TenantManager{
        tenants: make(map[string]*Tenant),
        users:   make(map[string]*User),
    }
}

// GenerateAPIKey creates a new API key
func GenerateAPIKey() string {
    bytes := make([]byte, 32)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)
}

// CreateTenant creates a new tenant
func (tm *TenantManager) CreateTenant(name string) (*Tenant, error) {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    
    tenant := &Tenant{
        ID:        GenerateAPIKey()[:16],
        Name:      name,
        APIKey:    GenerateAPIKey(),
        Databases: []string{"default"},
        Quota: Quota{
            MaxNodes:   1000000,
            MaxEdges:   1000000,
            MaxQueries: 10000,
            StorageBytes: 10 * 1024 * 1024 * 1024, // 10GB
        },
    }
    
    tm.tenants[tenant.ID] = tenant
    return tenant, nil
}

// CreateUser creates a new user for a tenant
func (tm *TenantManager) CreateUser(tenantID, username string, role Role) (*User, error) {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    
    if _, exists := tm.tenants[tenantID]; !exists {
        return nil, fmt.Errorf("tenant not found")
    }
    
    user := &User{
        ID:       GenerateAPIKey()[:16],
        Username: username,
        TenantID: tenantID,
        Role:     role,
        APIKey:   GenerateAPIKey(),
    }
    
    tm.users[user.ID] = user
    return user, nil
}

// ValidateAPIKey checks if an API key is valid and returns user and tenant
func (tm *TenantManager) ValidateAPIKey(apiKey string) (*User, *Tenant, error) {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    // Check if API key belongs to a user
    for _, user := range tm.users {
        if user.APIKey == apiKey {
            tenant, exists := tm.tenants[user.TenantID]
            if !exists {
                return nil, nil, fmt.Errorf("tenant not found")
            }
            return user, tenant, nil
        }
    }
    
    // Check if API key belongs to a tenant
    for _, tenant := range tm.tenants {
        if tenant.APIKey == apiKey {
            return nil, tenant, nil
        }
    }
    
    return nil, nil, fmt.Errorf("invalid API key")
}

// CheckPermission verifies if user has permission for an action
func (tm *TenantManager) CheckPermission(user *User, action string) bool {
    if user == nil {
        return false
    }
    
    switch user.Role {
    case RoleAdmin:
        return true
    case RoleWriter:
        return action != "delete_tenant" && action != "create_user"
    case RoleReader:
        return action == "read" || action == "query"
    default:
        return false
    }
}

// GetTenant returns tenant by ID
func (tm *TenantManager) GetTenant(id string) (*Tenant, error) {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    tenant, exists := tm.tenants[id]
    if !exists {
        return nil, fmt.Errorf("tenant not found")
    }
    return tenant, nil
}

// ListTenants returns all tenants
func (tm *TenantManager) ListTenants() []*Tenant {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    tenants := make([]*Tenant, 0, len(tm.tenants))
    for _, tenant := range tm.tenants {
        tenants = append(tenants, tenant)
    }
    return tenants
}
