package main

type User struct {
	Name string
	Age  int
}



type Repo interface {
	Get(id int) User
}

const MaxRetries = 3
var version = "1.0"

func main() {}

func (u *User) Greet() string { return "hi" }