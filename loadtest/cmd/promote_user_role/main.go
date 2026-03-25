package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	var (
		dsn      = flag.String("dsn", "root:2FaJkFtkMkC3yeBK@tcp(127.0.0.1:3306)/meshcart_user?charset=utf8mb4&parseTime=True&loc=Local", "mysql dsn")
		username = flag.String("username", "", "target username")
		role     = flag.String("role", "superadmin", "target role")
	)
	flag.Parse()

	if *username == "" {
		fmt.Fprintln(os.Stderr, "username is required")
		os.Exit(1)
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ping db failed: %v\n", err)
		os.Exit(1)
	}

	res, err := db.Exec("UPDATE users SET role = ? WHERE username = ?", *role, *username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update role failed: %v\n", err)
		os.Exit(1)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read rows affected failed: %v\n", err)
		os.Exit(1)
	}
	if rows == 0 {
		fmt.Fprintf(os.Stderr, "user %q not found\n", *username)
		os.Exit(1)
	}

	var (
		id       int64
		outUser  string
		outRole  string
	)
	if err := db.QueryRow("SELECT id, username, role FROM users WHERE username = ?", *username).Scan(&id, &outUser, &outRole); err != nil {
		fmt.Fprintf(os.Stderr, "query user failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("updated user: id=%d username=%s role=%s\n", id, outUser, outRole)
}
