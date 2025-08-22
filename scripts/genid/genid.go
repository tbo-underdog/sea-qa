package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

type Out struct {
	Vars map[string]string `json:"vars,omitempty"`
}

func main() {
	rand.Seed(time.Now().UnixNano())
	id := 100000000 + rand.Intn(900000000) // 9-digit
	_ = json.NewEncoder(os.Stdout).Encode(Out{
		Vars: map[string]string{"PET_ID": fmt.Sprintf("%d", id)},
	})
}
