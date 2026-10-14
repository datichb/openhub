// Commande todo : ajoute et liste des tâches gardées dans todo.txt.
package main

import (
	"fmt"
	"os"
	"strings"

	"example.com/todo/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage : todo add <tâche> | todo list")
		os.Exit(2)
	}
	s := store.Open("todo.txt")
	switch os.Args[1] {
	case "add":
		if err := s.Add(strings.Join(os.Args[2:], " ")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "list":
		tasks, err := s.List()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for i, t := range tasks {
			fmt.Printf("%d. %s\n", i+1, t)
		}
	default:
		fmt.Fprintln(os.Stderr, "commande inconnue :", os.Args[1])
		os.Exit(2)
	}
}
