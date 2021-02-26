# slowql-replayer

A tool to replay queries from a slow query log file.

## Installation

```
$ go install github.com/devops-works/slowql/cmd/slowql-replayer
```

## Usage

```
Usage of ./slowql-replayer:
  -db string
        Name of the database to use
  -dry
        Replay the requests but don't write in the database
  -f string
        Slow query log file to use
  -h string
        Addres of the database, with IP and port
  -k string
        Kind of the database (mysql, mariadb...)
  -l string
        Logging level (default "info")
  -p    Use a password to connect to database
  -u string
        User to use to connect to database
```

A typical example might be:

```
$ ./slowql-replayer -db users -f samples/mariadb_slowquery.log -h 127.0.0.1:3306 -k mariadb -u ezekiel -p
```

By adding `-dry`, it will not send the queries to the database, but just sleep the required time to simulate the real-life scenario.

At the end, a short report is displayed:

```
+---------+---------+---------+--------+---------------+
|   DB    | DRY RUN | QUERIES | ERRORS |   DURATION    |
+---------+---------+---------+--------+---------------+
| mariadb |  false  |  1395   |   17   | 49.744778454s |
+---------+---------+---------+--------+---------------+
```

The following table shows all the accepted values for `-k`

|        Database        | `-k` value |
| :--------------------: | :--------: |
|         MySQL          |   mysql    |
|        MariaDB         |  mariadb   |
| Percona XtraDB Cluster |    pcx     |

## Supported databases

We successfully tested `slowql-replay` on:

* Locally
    - [X] MySQL
    - [X] MariaDB
    - [ ] MongoDB
    - [ ] PostgreSQL
    - [ ] Percona-db
    - [X] Percona-cluster

* In real life
    - [ ] MySQL
    - [ ] MariaDB
    - [ ] MongoDB
    - [ ] PostgreSQL
    - [ ] Percona-db
    - [ ] Percona-cluster

**Note:** `slowql-replayer` relies heavily on the `slowql` package, so if a database is missing in the package, it will not be present in the replayer.

## License

MIT