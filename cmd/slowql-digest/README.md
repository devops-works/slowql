# slowql-digest

A tool to digest, analyze and show stats about slow query logs. Similar to `pt-query-digest`, but a little bit (9x) faster.

## Installation

There is multiple ways to get `slowql-digest`.

### By using `go install`

```
$ go install github.com/devops-works/slowql/cmd/slowql-digest
```

### By cloning the repo and building it

```
$ git clone https://github.com/devops-works/slowql
$ cd slowql/
$ make digest
```

A binary called `digest` will be created at the root of the repo, under `bin/`.

(`go` is required!)

### By downloading the pre-built binary

You can find the latest version in the [releases](https://github.com/devops-works/slowql/releases)

## Usage

```
Usage of digest:
  -dec
        Sort by decreasing order
  -f string
        Slow query log file to digest (required)
  -k string
        Database kind. Use ? to see all the available values  (required)
  -l string
        Log level (default "info")
  -no-cache
        Do not use cache, if cache exists
  -sort-by string
        How to sort queries. Use ? to see all the available values (default "random")
  -top int
        Top queries to show (default 3)
```

A minimal example is:

```
$ ./digest -f my-slowql.log -k mariadb
```

This will digest `my-slowql.log` which is a MariaDB-based slow query log.

## Ordering

The options `-sort-by` and `-top` allow respectively to sort the results by a certain field (number of calls of the the query, bytes sent, concurrency...) and to set a specific number of queries to show (the top 10 for example.)

By default, they will be displayed in an increasing order (lower first). the option `-dec` allows to reverse the order.

## Caching

By default, `digest` will try to read from a cache located at the same emplacement than your slow query log file. If it does not exist, it will create one in order to avoid doing all the slow calculations multiple times.

You can disable the cache with the option `-no-cache`.

## Docker

The file `Dockerfile.digest` allows you to build the Docker image of `digest`:

```
$ docker build -f Dockerfile.digest -t dw/digest .
```

By default, `digest` looks for file called `slowquery.log` at `/log`. So you can provide the file by sharing it via a volume, and then give the arguments:

```
$ docker run --rm -v /path/to/slowquery/file/local-slowquery.log:/log/slowquery.log  dw/digest -k mysql
```

## Supported databases

We successfully tested `digest` on:

- [X] MySQL
- [X] MariaDB
- [ ] Percona-db
- [X] Percona-cluster

**Note:** `slowql-digest` relies heavily on the `slowql` package, so if a database is missing in the package, it will not be present in the digester.

## License

MIT