package main

import (
	"fmt"

	"github.com/dkeye/Voice/internal/transport/http"
)

func main() {
	r := http.SetupRouter()

	addr := ":8080"
	fmt.Println("Voice server started at", addr)
	if err := r.Run(addr); err != nil {
		panic(err)
	}
}
