package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

var users = map[int]User{
	1: {Name: "Alice", Age: 30},
	2: {Name: "Bob", Age: 25},
}
var nextID = 3

func listUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	json.NewEncoder(w).Encode(users)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id := parseID(r.URL.Path)
	user, ok := users[id]
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(user)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	users[nextID] = user
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"id": nextID})
	nextID++
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	id := parseID(r.URL.Path)
	if _, ok := users[id]; !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	users[id] = user
	json.NewEncoder(w).Encode(user)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	id := parseID(r.URL.Path)
	if _, ok := users[id]; !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	delete(users, id)
	w.WriteHeader(http.StatusNoContent)
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func parseID(path string) int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return 0
	}
	var id int
	for _, c := range parts[len(parts)-1] {
		if c >= '0' && c <= '9' {
			id = id*10 + int(c-'0')
		}
	}
	return id
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/") && r.URL.Path != "/users" {
			switch r.Method {
			case http.MethodGet:
				getUser(w, r)
			case http.MethodPut:
				updateUser(w, r)
			case http.MethodDelete:
				deleteUser(w, r)
			default:
				methodNotAllowed(w)
			}
			return
		}
		switch r.Method {
		case http.MethodGet:
			listUsers(w, r)
		case http.MethodPost:
			createUser(w, r)
		default:
			methodNotAllowed(w)
		}
	})
	http.ListenAndServe(":8080", mux)
}
