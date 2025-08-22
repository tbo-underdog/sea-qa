package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type In struct {
	Vars map[string]string `json:"vars"`
}
type Out struct {
	Vars    map[string]string `json:"vars,omitempty"`
	Request *struct {
		Headers map[string]string `json:"headers,omitempty"`
	} `json:"request,omitempty"`
}

func main() {
	var in In
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}
	token := fmt.Sprintf("token-%d", time.Now().Unix()) // fake
	out := Out{
		Request: &struct {
			Headers map[string]string `json:"headers,omitempty"`
		}{
			Headers: map[string]string{"Authorization": "Bearer " + token},
		},
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}
