# slowql-replayer

A tool to replay queries from a slow query log file.

## Installation

There is multiple ways to get `slowql-replayer`.

### By using `go install`

```
$ go install github.com/devops-works/slowql/cmd/slowql-replayer
```

### By cloning the repo and building it

```
$ git clone https://github.com/devops-works/slowql
$ cd slowql/
$ make replayer
```

A binary called `replayer` will be created at the root of the repo, under `bin/`.

(`go` is required!)

### By downloading the pre-built binary

You can find the latest version in the [releases](https://github.com/devops-works/slowql/releases)

## Usage

```
Usage of slowql-replayer:
  -db string
        Name of the database to use
  -f string
        Slow query log file to use
  -h string
        Addres of the database, with IP and port
  -hide-progress
        Hide progress bar while replaying
  -k string
        Kind of the database (mysql, mariadb...)
  -l string
        Logging level (default "info")
  -no-dry-run
        Replay the requests on the database for real
  -p    Use a password to connect to database
  -pprof string
        pprof server address
  -show-errors
        Show SQL errors when they occur
  -u string
        User to use to connect to database
  -w int
        Number of maximum simultaneous connections to database (default 100)
  -x float
        Speed factor (default 1)
```

A typical example might be:

```
$ ./slowql-replayer -u ezekiel -p -h 192.168.1.2:3306 -k mysql -f ~/files/databases/log/mysql.log -db mydb
```

By adding `-no-dry-run`, it will send the queries to the database for real.

At the end, a short report is displayed:

```
=-= Results =-=

Replay duration:  49.330633992s
Real duration:    49.327466s
Log file:         /home/ezekiel/files/databases/log/mysql.log
Dry run:          false
Workers:          100

Database
  ├─ kind:      mysql
  ├─ user:      ezekiel
  ├─ use pass:  true
  └─ address:   192.168.1.2:3306

Statistics
  ├─ Queries:                90004
  ├─ Errors:                 2
  ├─ Queries success rate:   99.9978%
  ├─ Speed factor:           1.0000
  ├─ Duration difference:    replayer took 3.167992ms more
  └─ Replayer speed:         -0.0064%
```

## Docker

Build the image (you need to be at the root of the repo):

```
$ docker build -f Dockerfile.replayer -t slowql-replayer
...
$ docker run --rm -it slowql-replayer [OPTIONS]
```

### Adjustments

#### Speed

You can also adjust a speed factor with the option `-x` which can receive a float. For example, if you set the speed factor to `2`, it will replay the queries twice as fast. On the ortherhand, if you set it to `0.5`, it will replay twice as slow.

#### Concurrency

Also, you can specify the number of workers that will request the database is simultaneously with `-w`. This way, you can simulate concurrency. The default value is `100`.

#### Progress bar

If the progress bar bothers you, you can hide it with `-hide-progress`.

#### Databases kinds

The following table shows all the accepted values for `-k`

|        Database        | `-k` value |
| :--------------------: | :--------: |
|         MySQL          |   mysql    |
|        MariaDB         |  mariadb   |
| Percona XtraDB Cluster |    pxc     |

## Supported databases

We successfully tested `slowql-replay` on:

* Locally
    - [X] MySQL
    - [X] MariaDB
    - [ ] Percona-db
    - [X] Percona-cluster

* In real life
    - [ ] MySQL
    - [ ] MariaDB
    - [ ] Percona-db
    - [ ] Percona-cluster

**Note:** `slowql-replayer` relies heavily on the `slowql` package, so if a database is missing in the package, it will not be present in the replayer.

## License

MIT