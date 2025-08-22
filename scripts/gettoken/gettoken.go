package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Out struct {
	Request string `json:"request,omitempty"` // add this if you use it
	Token   string `json:"token,omitempty"`
	Error   string `json:"error,omitempty"`
}

func main() {
	req := ""
	if len(os.Args) > 1 {
		req = os.Args[1]
	}

	o := Out{
		Request: req,        // remove this line if you don’t want Request
		Token:   "example",  // TODO: replace with real token logic
	}
	b, _ := json.Marshal(o)
	fmt.Println(string(b))
}
