SET SESSION statement_timeout = 3000;
SET SESSION lock_timeout = 3000;

ALTER TABLE "warehouse"."storage_locations" ADD CONSTRAINT "ck_capacity" CHECK((total_capacity >= used_capacity)) NOT VALID;

ALTER TABLE "warehouse"."storage_locations" VALIDATE CONSTRAINT "ck_capacity";

