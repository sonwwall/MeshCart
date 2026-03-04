package main

import (
	"log"
	user "meshcart/kitex_gen/meshcart/user/userservice"
)

func main() {
	svr := user.NewServer(new(UserServiceImpl))

	err := svr.Run()

	if err != nil {
		log.Println(err.Error())
	}
}
