package main

import (
	"fmt"
	"os"
	"time"

	"github.com/devops-works/slowql"
	"github.com/sirupsen/logrus"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s file.log\n", os.Args[0])
		os.Exit(1)
	}

	fd, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	p := slowql.NewParser(fd)

	time.Sleep(time.Second)
	_, err = p.GetNext()
	if err != nil {
		logrus.Error(err)
	}
}
