# trek

Trek is a tool that assists in PostgreSQL schema development and deployment. Trek provides a unified and simple workflow for developers to change the database schema and commit versioned migration files to a VCS such as git. Trek also helps with rolling out those migrations to a live database.

A basic trek workflow looks like this:
- Initialize a trek directory `mkdir my_database; cd my_database; trek init`
- Change the database schema by using the [pgModeler](https://github.com/pgmodeler/pgmodeler) GUI.
- Run trek to generate a new migration file. The migration file will contain all changes since the last version as PostgreSQL DDL (ALTER TABLE, etc.). The migration file is stored as file in the `my_database/migrations` folder.
- Commit the migration file to an VCS like git. The changed pgModeler schema and the migration SQL file may be code-reviewed.
- Deploy the latest version of `my_database` to a live database by running `trek apply`. Trek uses [golang-migrate](https://github.com/golang-migrate/migrate) to keep track of the latest version deployed to a database, and to deploy new migration files to the database. (You may skip `trek apply` and integrate with `golang-migrate` manually.)

A developer could run `trek generate --stdout` to continuously watch the pgModeler model for updates (on save) and write PostgreSQL DDL to stdout.

Trek is an opinionated tool. We built it to do one thing and do it well: convert pgModeler schema design into versioned migrations. Trek is exclusively designed for use with PostgreSQL, and we firmly intend to maintain this focus. As such, we will not be considering or adding support for any other database systems.

## Compatibility

Trek works with PostgreSQL version 13 or newer. We build trek with the latest stable version of pgModeler.

## Installation

```bash
go install github.com/printeers/trek@latest
```

Or use a [released docker image](https://github.com/printeers/trek/pkgs/container/trek).

## Dependencies

Running `trek generate` requires to have `migra` and `pgmodeler-cli` installed. Trek will try to locate these in the `$PATH` of the user.

If `migra` cannot be found, trek will try use versions of this binary that is embedded into the trek build. The user can skip searching for `migra` in the path by setting `TREK_FORCE_EMBEDDED_MIGRA`. For example:

```bash
trek --force-embedded-migra=true generate --stdout
```

_Note that the embedded migra is [built using a patched schemainspect library](internal/embedded/migra/build-migra.Dockerfile), which is [awaiting upstream merge](https://github.com/djrobstep/schemainspect/pull/67)._

If you have trouble setting up pgmodeler-cli

## MacOS

If you're running on a mac (for which the embedded `migra` doesn't work), you may want to consider using `docker` to run trek under `linux/amd64` (Rosetta emulation).

## Docker

To run `trek` in docker:

```bash
docker run -v ./:/data ghcr.io/printeers/trek:latest-pgmodeler trek ...
```

## Setup

Create `trek.yaml`:

```yaml
model_name: <model_name>
db_name: <db_name>
db_users:
  - <db_user_1>
  - <db_user_2>
```

Create `<model_name>.dbm` using pgModeler.

## Creating migrations

`trek generate some-migration`

Use the `--dev` flag to continuously watch for file changes.

## Applying the migrations

Take a look at the `example/` directory.

## History

`trek` was originally developed by [Stack11](https://github.com/stack11). In april 2023 [Printeers](https://printeers.com) adopted the project for further development and maintenance.
