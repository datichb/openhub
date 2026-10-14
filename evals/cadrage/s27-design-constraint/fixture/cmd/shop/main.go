// Commande shop : valide une commande d'exemple.
package main

import (
	"fmt"
	"log"

	"example.com/shop/internal/mail"
	"example.com/shop/internal/order"
)

func main() {
	_ = &mail.Client{Addr: "localhost:25", From: "shop@example.com"}
	o := &order.Order{ID: "c-1", Customer: "ana@example.com", Lines: []order.Line{{SKU: "A", Quantity: 1}}}
	if err := o.Validate(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("commande validée :", o.ID)
}
