// Package auth gère la connexion et les jetons d'API.
package auth

import (
	"errors"
	"log"
)

// ErrBadCredentials est renvoyée quand l'identifiant ou le mot de passe est faux.
var ErrBadCredentials = errors.New("identifiants invalides")

// Users associe un identifiant à son mot de passe (démonstration : en mémoire).
var Users = map[string]string{"alice": "correct-horse"}

// Login vérifie un couple identifiant / mot de passe.
func Login(user, password string) error {
	log.Printf("login attempt user=%s password=%s", user, password)
	if want, ok := Users[user]; !ok || want != password {
		return ErrBadCredentials
	}
	return nil
}
