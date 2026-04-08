package core

import (
    "fmt"

    "github.com/dgraph-io/badger/v4"
)

// Column family names (as prefixes for keys)
const (
    CFNodesCurrent  = "nc:"  // node current
    CFEdgesCurrent  = "ec:"  // edge current
    CFNodesHistory  = "nh:"  // node history
    CFEdgesHistory  = "eh:"  // edge history
)

type StorageEngine struct {
    db *badger.DB
}

// NewStorageEngine creates a new Badger storage engine
func NewStorageEngine(dataDir string) (*StorageEngine, error) {
    opts := badger.DefaultOptions(dataDir)
    opts.Logger = nil // Disable logging for tests
    
    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    return &StorageEngine{
        db: db,
    }, nil
}

// Put stores a key-value pair
func (s *StorageEngine) Put(cfName string, key, value []byte) error {
    fullKey := append([]byte(cfName), key...)
    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set(fullKey, value)
    })
}

// Get retrieves a value
func (s *StorageEngine) Get(cfName string, key []byte) ([]byte, error) {
    fullKey := append([]byte(cfName), key...)
    var result []byte
    
    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get(fullKey)
        if err == badger.ErrKeyNotFound {
            return nil
        }
        if err != nil {
            return err
        }
        
        result, err = item.ValueCopy(nil)
        return err
    })
    
    if err != nil {
        return nil, err
    }
    return result, nil
}

// WriteBatch performs multiple writes atomically (Badger handles this via transactions)
func (s *StorageEngine) WriteBatch(updates map[string]map[string][]byte) error {
    return s.db.Update(func(txn *badger.Txn) error {
        for cfName, kvPairs := range updates {
            for key, value := range kvPairs {
                fullKey := append([]byte(cfName), []byte(key)...)
                if err := txn.Set(fullKey, value); err != nil {
                    return err
                }
            }
        }
        return nil
    })
}

// Close closes the database
func (s *StorageEngine) Close() error {
    return s.db.Close()
}

// GetStats returns database statistics
func (s *StorageEngine) GetStats() map[string]interface{} {
    stats := map[string]interface{}{}
    if err := s.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()
        
        var count int
        for it.Rewind(); it.Valid(); it.Next() {
            count++
        }
        stats["key_count"] = count
        return nil
    }); err != nil {
        stats["error"] = err.Error()
    }
    return stats
}
