package main

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
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

	fmt.Println("Operations simulations...")
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Start()
	time.Sleep(4 * time.Second)
	s.Stop()

	q, err := p.GetNext()
	if err != nil {
		logrus.Error(err)
	}

	fmt.Printf("%v\n", q)
}
