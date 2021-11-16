# trek

## Requirements
At least version 13 of postgres is needed.

## Installation
`go install .`

## Setup
Create `config.yaml`:
```yaml
model_name: <model_name>
db_name: <db_name>
db_users:
  - <db_user_1>
  - <db_user_2>
```

Create `<model_name>.dbm` using pgModeler.

## Initial migration
`trek generate -i`

## Further migrations
`trek generate 002_some_migration`

## Applying the migrations
Take a look at the `example/` directory.
