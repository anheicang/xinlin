package main

import (
	"fmt"
	"log"

	"github.com/anheicang/xinlin/base/phone"
	"github.com/anheicang/xinlin/base/points"
)

func main() {
	amount, err := points.Parse("12.34")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(amount, points.Format(amount))
	fmt.Println(phone.Normalize("+86 138-0013-8000"))
}
