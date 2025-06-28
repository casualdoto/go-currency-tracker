package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Go Currency Monitor API started on :8080")
	http.ListenAndServe(":8080", nil)
}
