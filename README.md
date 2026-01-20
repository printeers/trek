# trek

Trek is a tool that assists in PostgreSQL schema development and deployment. Trek provides a unified and simple workflow for developers to change the database schema and commit versioned migration files to a VCS such as git. Trek also helps with rolling out those migrations to a live database.

A basic trek workflow looks like this:
- Initialize a trek directory `mkdir my_database; cd my_database; trek init`
- Change the database schema by using the [pgModeler](https://github.com/pgmodeler/pgmodeler) GUI.
- Run trek to generate a new migration file. The migration file will contain all changes since the last version as PostgreSQL DDL (ALTER TABLE, etc.). The migration file is stored as file in the `my_database/migrations` folder.
- Commit the migration file to an VCS like git. The changed pgModeler schema and the migration SQL file may be code-reviewed.
- Deploy the latest version of `my_database` to a live database by running `trek apply`. Trek uses [golang-migrate](https://github.com/golang-migrate/migrate) to keep track of the latest version deployed to a database, and to deploy new migration files to the database. (You may skip `trek apply` and integrate with `golang-migrate` manually.)

A developer could run `trek generate --dev --stdout` to continuously watch the pgModeler model for updates (on save) and write PostgreSQL DDL to stdout.

Trek is an opinionated tool. We built it to do one thing and do it well: convert pgModeler schema design into versioned migrations. Trek is exclusively designed for use with PostgreSQL, and we firmly intend to maintain this focus. As such, we will not be considering or adding support for any other database systems.

## Compatibility

Trek tries to stay up-to-date with the latest stable version of PostgreSQL. We build trek with the latest stable version of pgModeler.

| Trek version | PostgreSQL | pgModeler |
|--------------|------------|-----------|
| 0.12.x       | 18         | 1.2       |

We develop and test Trek on Linux. Trek may work on other platforms via Docker.

## Installation

```bash
go install github.com/printeers/trek@latest
```

Or use a [released docker image](https://github.com/printeers/trek/pkgs/container/trek).

## Dependencies

Trek requires to have `psql`, `pg_dump` and `pgmodeler-cli` installed. Trek will try to locate these in the `$PATH` of the user.

## MacOS / Windows

If you're running on MacOS or Windows, then the embedded `migra` doesn't work. Either install migra manually or use `docker` to run trek under `linux/amd64` (Rosetta emulation).

## Docker

To run `trek` in docker:

```bash
cd your_trek_working_directory
docker run -v ./:/data ghcr.io/printeers/trek:latest-pgmodeler trek ...
```

## Setup a trek working directory manually

We recommended to use `trek init`, but you may setup a working directory manually.

Inside a directory create `trek.yaml`:

```yaml
model_name: <model_name>
db_name: <db_name>
roles:
  - name: db_user_1
  - name: db_user_2
```

Create `<model_name>.dbm` using pgModeler.

## Generating a new migration

`trek generate some-migration`

Use the `--dev` flag to continuously watch for file changes. Use the `--stdout` flag to write migrations to stdout. You must omit the migration name when using `--stdout`.

## Applying the migrations

Take a look at the `example/` directory.

## History

`trek` was originally developed at [Stack11](https://github.com/stack11). In april 2023 [Printeers](https://printeers.com) adopted the project for further development and maintenance.

## License

Copyright 2021 [The Trek Authors](https://github.com/printeers/trek/graphs/contributors). Available under the [AGPL 3.0 license](./LICENSE).
