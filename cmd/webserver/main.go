package main

import (
    "log"
    "net/http"
)

func main() {
    fs := http.FileServer(http.Dir("./webui"))
    http.Handle("/", fs)
    
    log.Println("ChronosDB Web UI starting on http://localhost:8081")
    log.Fatal(http.ListenAndServe(":8081", nil))
}
