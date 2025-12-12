SET SESSION statement_timeout = 3000;
SET SESSION lock_timeout = 3000;

CREATE TABLE "factory"."machines" (
	"name" text COLLATE "pg_catalog"."default" NOT NULL,
	"toys_produced" bigint NOT NULL
);

CREATE TABLE "warehouse"."storage_locations" (
	"shelf" bigint NOT NULL,
	"total_capacity" bigint NOT NULL,
	"used_capacity" bigint NOT NULL,
	"current_toy_type" text COLLATE "pg_catalog"."default" NOT NULL
);

