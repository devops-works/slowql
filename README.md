# slowql

A slow query logs parser in Golang.

- [Gettin' started](#gettin-started)
- [Basic usage](#basic-usage)
- [Performances](#performances)
- [Notes](#notes)
  - [Tested databases](#tested-databases)
  - [Internal](#internal)
- [Contributing](#contributing)
- [License](#license)

## Gettin' started

```
go get github.com/devops-works/slowql
```

## Basic usage

To put in a nutshell, you can use it as follows:

```go
package main

import (
    "fmt"

    "github.com/devops-works/slowql"
)

func main() {
    // Imagine that fd is an io.Reader of your slow query logs file...

    // Create the parser
    p := slowql.NewParser(fd)

    // Get the next query from the log
    q, err := p.GetNext()
	if err != nil {
		panic(err)
	}

    // Do your stuff, for example:
    fmt.Printf("at %d, %s did the request: %s\n", q.Time, q.User, q.Query)

    fp, err := q.Fingerprint()
    if err != nil {
        panic(err)
    }
    fmt.Printf("the query fingerprint is: %s\n", fp)
}
```

## Performances

Running the example given in cmd/ without any `fmt.Printf` against a 292MB slow query logs from a MySQL database provides the following output:

```
parsed 278077 queries in 8.099622786s
```

which is approx. **34760 queries/seconds**.

## Notes

### Tested databases

Not all kind of slow query logs have been tested yet:

- [X] MySQL
- [X] MariaDB
- [ ] Percona-db
- [ ] Percona-cluster (pxc)

### Internal

In the background, when you call `slowql.NewParser`, it starts two goroutines. The first one will read the file line by line and assemble them to create "blocs" composed of hearders (starting with #) and requests. Once created, those blocs are send through a channel.
The second goroutine will intercept blocs and populate a `Query` struct with the values that matches its rules. Once the bloc parsed, the `Query` object is sent through another channel which is bufferized. Actually, it has a buffer size of `parser.StackSize`. When you call `parser.GetNext()`, you simply get the first value of the channel's buffer.

The goroutines only stop when there is nothing more to read from the log file. So once the buffer is full, only a little part of CPU workload will be used to keep the buffer full each time a value is extracted.

## Contributing

Issues and pull request are welcomed ! If you found a bug or want to help and improve this package don't hesitate to fork it or open an issue :smile:

## License

MIT