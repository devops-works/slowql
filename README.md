# slowql

A slow query logs parser in Golang.

- [slowql](#slowql)
  - [Gettin' started](#gettin-started)
  - [Basic usage](#basic-usage)
  - [Performances](#performances)
  - [Associated tools](#associated-tools)
  - [Notes](#notes)
    - [Tested databases](#tested-databases)
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

    srv := p.GetServerMeta()
    fmt.Printf("the SQL server listens to port %d", srv.Port)
}
```

## Performances

Running the example given in cmd/ without any `fmt.Printf` against a 292MB slow query logs from a MySQL database provides the following output:

```
parsed 278077 queries in 8.099622786s
```

which is approx. **34760 queries/second** (Intel i5-8250U (8) @ 3.400GHz).

## Associated tools

With this package we created some tools:

* [slowql-replayer](https://github.com/devops-works/slowql/tree/develop/cmd/slowql-replayer): replay and benchmark queries from a slow query log
* [slowql-digest](https://github.com/devops-works/slowql/tree/develop/cmd/slowql-digest): digest and analyze slow query logs. Similar to `pt-query-digest`, but faster. :upside_down_face:

## Notes

### Tested databases

Not all kind of slow query logs have been tested yet:

- [X] MySQL
- [X] MariaDB
- [ ] Percona-db
- [X] Percona-cluster (pxc)
- [ ] PostgreSQL

## Contributing

Issues and pull requests are welcomed ! If you found a bug or want to help and improve this package don't hesitate to fork it or open an issue :smile:

## License

MIT