# trek

## Compatibility

Trek works with PostgreSQL version 13 or newer. We build trek with the latest stable version of pgModeler.

## Installation

```bash
go install github.com/printeers/trek@latest
```

Or use a [released docker image](https://github.com/printeers/trek/pkgs/container/trek).

## Dependencies

Trek depends on `migra` and `pgmodeler-cli`. Trek will try to locate these in the `$PATH` of the user.

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
